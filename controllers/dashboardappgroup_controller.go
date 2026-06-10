package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dashboardv1alpha1 "github.com/yamlwrangler/app-dashboard-operator/api/v1alpha1"
)

const (
	// Annotation keys used by the dashboard
	AnnotationEnabled      = "dashboard.yamlwrangler.com/enabled"
	AnnotationDisplayName  = "dashboard.yamlwrangler.com/display-name"
	AnnotationCategory     = "dashboard.yamlwrangler.com/category"
	AnnotationDescription  = "dashboard.yamlwrangler.com/description"
	AnnotationAppGroup     = "dashboard.yamlwrangler.com/app-group"
	AnnotationPrimaryRoute = "dashboard.yamlwrangler.com/primary-route"
	AnnotationCustomLinks  = "dashboard.yamlwrangler.com/custom-links"

	// Label key for filtering deployments (labels have same restrictions as annotations in k8s)
	LabelEnabled = "dashboard.yamlwrangler.com/enabled"
)

// DashboardAppGroupReconciler reconciles a DashboardAppGroup object
type DashboardAppGroupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=dashboard.yamlwrangler.com,resources=dashboardappgroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dashboard.yamlwrangler.com,resources=dashboardappgroups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dashboard.yamlwrangler.com,resources=dashboardappgroups/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *DashboardAppGroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the DashboardAppGroup instance
	appGroup := &dashboardv1alpha1.DashboardAppGroup{}
	err := r.Get(ctx, req.NamespacedName, appGroup)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, could have been deleted
			logger.Info("DashboardAppGroup resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object
		logger.Error(err, "Failed to get DashboardAppGroup")
		return ctrl.Result{}, err
	}

	// Ensure namespace is labeled for dashboard discovery
	err = r.ensureNamespaceLabeled(ctx, appGroup.Namespace)
	if err != nil {
		logger.Error(err, "Failed to label namespace")
		return ctrl.Result{}, err
	}

	// Ensure ConfigMap exists for this namespace
	err = r.ensureConfigMapExists(ctx, appGroup.Namespace)
	if err != nil {
		logger.Error(err, "Failed to ensure ConfigMap exists")
		return ctrl.Result{}, err
	}

	// Find matching deployments
	matchedDeployments, err := r.findMatchingDeployments(ctx, appGroup)
	if err != nil {
		logger.Error(err, "Failed to find matching deployments")
		return ctrl.Result{}, err
	}

	logger.Info("Found matching deployments", "count", len(matchedDeployments), "deployments", matchedDeployments)

	// Apply labels to matched deployments if autoLabel is enabled
	if appGroup.Spec.AutoLabel {
		err = r.labelDeployments(ctx, appGroup, matchedDeployments)
		if err != nil {
			logger.Error(err, "Failed to label deployments")
			return ctrl.Result{}, err
		}
	}

	// Update status
	appGroup.Status.MatchedDeployments = matchedDeployments
	appGroup.Status.LastUpdated = metav1.Now()

	// Set condition
	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "DeploymentsLabeled",
		Message:            fmt.Sprintf("Successfully labeled %d deployments", len(matchedDeployments)),
		LastTransitionTime: metav1.Now(),
	}

	// Update or append condition
	found := false
	for i, c := range appGroup.Status.Conditions {
		if c.Type == "Ready" {
			appGroup.Status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		appGroup.Status.Conditions = append(appGroup.Status.Conditions, condition)
	}

	err = r.Status().Update(ctx, appGroup)
	if err != nil {
		logger.Error(err, "Failed to update DashboardAppGroup status")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully reconciled DashboardAppGroup", "name", appGroup.Name, "namespace", appGroup.Namespace)

	// Requeue after 5 minutes to check for new deployments
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// findMatchingDeployments finds all deployments that match the selector
func (r *DashboardAppGroupReconciler) findMatchingDeployments(ctx context.Context, appGroup *dashboardv1alpha1.DashboardAppGroup) ([]string, error) {
	logger := log.FromContext(ctx)
	var matched []string

	// List all deployments in the namespace
	deploymentList := &appsv1.DeploymentList{}
	err := r.List(ctx, deploymentList, client.InNamespace(appGroup.Namespace))
	if err != nil {
		return nil, err
	}

	selector := appGroup.Spec.Selector

	// Match by pattern
	if selector.MatchPattern != "" {
		pattern, err := regexp.Compile(selector.MatchPattern)
		if err != nil {
			logger.Error(err, "Invalid regex pattern", "pattern", selector.MatchPattern)
			return nil, err
		}

		for _, deployment := range deploymentList.Items {
			if pattern.MatchString(deployment.Name) {
				matched = append(matched, deployment.Name)
			}
		}
	}

	// Match by explicit names
	if len(selector.MatchNames) > 0 {
		nameSet := make(map[string]bool)
		for _, name := range selector.MatchNames {
			nameSet[name] = true
		}

		for _, deployment := range deploymentList.Items {
			if nameSet[deployment.Name] {
				// Avoid duplicates
				if !contains(matched, deployment.Name) {
					matched = append(matched, deployment.Name)
				}
			}
		}
	}

	// Match by labels
	if len(selector.MatchLabels) > 0 {
		for _, deployment := range deploymentList.Items {
			if matchesLabels(deployment.Labels, selector.MatchLabels) {
				// Avoid duplicates
				if !contains(matched, deployment.Name) {
					matched = append(matched, deployment.Name)
				}
			}
		}
	}

	return matched, nil
}

// labelDeployments applies dashboard annotations to matched deployments
func (r *DashboardAppGroupReconciler) labelDeployments(ctx context.Context, appGroup *dashboardv1alpha1.DashboardAppGroup, deploymentNames []string) error {
	logger := log.FromContext(ctx)

	for _, name := range deploymentNames {
		deployment := &appsv1.Deployment{}
		err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: appGroup.Namespace}, deployment)
		if err != nil {
			logger.Error(err, "Failed to get deployment", "deployment", name)
			continue
		}

		// Initialize annotations if nil
		if deployment.Annotations == nil {
			deployment.Annotations = make(map[string]string)
		}

		// Initialize labels if nil
		if deployment.Labels == nil {
			deployment.Labels = make(map[string]string)
		}

		// Apply both label and annotation for enabled
		// Label is used by the plugin to watch/filter deployments
		// Annotation is used to store additional metadata
		deployment.Labels[LabelEnabled] = "true"
		deployment.Annotations[AnnotationEnabled] = "true"
		// Note: We don't set display-name so each deployment uses its own name
		deployment.Annotations[AnnotationCategory] = appGroup.Spec.Category

		logger.Info("Setting labels and annotations",
			"deployment", name,
			"labelKey", LabelEnabled,
			"labelValue", deployment.Labels[LabelEnabled],
			"annotationKey", AnnotationEnabled,
			"annotationValue", deployment.Annotations[AnnotationEnabled])

		if appGroup.Spec.Description != "" {
			deployment.Annotations[AnnotationDescription] = appGroup.Spec.Description
		}

		// Set app-group to the AppGroup name (this groups them together)
		deployment.Annotations[AnnotationAppGroup] = appGroup.Name

		// Only set primary-route annotation on the deployment that matches the route name
		// This ensures only the primary deployment gets the route URL
		if appGroup.Spec.PrimaryRoute != "" {
			// Check if this deployment's name matches the route name pattern
			// Routes typically match deployment names (e.g., plane-web route -> plane-web-wl deployment)
			routeName := appGroup.Spec.PrimaryRoute
			deploymentName := deployment.Name

			// Match if deployment name starts with route name followed by a hyphen or is exact match
			// This prevents "plane-web" from matching "plane-api-wl"
			if deploymentName == routeName ||
				(len(deploymentName) > len(routeName) &&
					deploymentName[:len(routeName)] == routeName &&
					deploymentName[len(routeName)] == '-') {
				deployment.Annotations[AnnotationPrimaryRoute] = appGroup.Spec.PrimaryRoute
			}
		}

		// Serialize custom links if present
		if len(appGroup.Spec.CustomLinks) > 0 {
			linksJSON, err := json.Marshal(appGroup.Spec.CustomLinks)
			if err != nil {
				logger.Error(err, "Failed to marshal custom links", "deployment", name)
			} else {
				deployment.Annotations[AnnotationCustomLinks] = string(linksJSON)
			}
		}

		// Update the deployment
		err = r.Update(ctx, deployment)
		if err != nil {
			logger.Error(err, "Failed to update deployment", "deployment", name)
			return err
		}

		logger.Info("Successfully labeled deployment", "deployment", name, "appGroup", appGroup.Name)
	}

	return nil
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func matchesLabels(deploymentLabels, selectorLabels map[string]string) bool {
	for key, value := range selectorLabels {
		if deploymentLabels[key] != value {
			return false
		}
	}
	return true
}

// ensureNamespaceLabeled ensures the namespace has the dashboard enabled label
func (r *DashboardAppGroupReconciler) ensureNamespaceLabeled(ctx context.Context, namespaceName string) error {
	logger := log.FromContext(ctx)

	namespace := &corev1.Namespace{}
	err := r.Get(ctx, types.NamespacedName{Name: namespaceName}, namespace)
	if err != nil {
		return fmt.Errorf("failed to get namespace: %w", err)
	}

	// Check if already labeled
	if namespace.Labels != nil && namespace.Labels[NamespaceEnabledLabel] == "true" {
		logger.Info("Namespace already labeled for dashboard", "namespace", namespaceName)
		return nil
	}

	// Initialize labels if nil
	if namespace.Labels == nil {
		namespace.Labels = make(map[string]string)
	}

	// Add the label
	namespace.Labels[NamespaceEnabledLabel] = "true"

	err = r.Update(ctx, namespace)
	if err != nil {
		return fmt.Errorf("failed to update namespace labels: %w", err)
	}

	logger.Info("Successfully labeled namespace for dashboard", "namespace", namespaceName)
	return nil
}

// ensureConfigMapExists ensures the dashboard ConfigMap exists for the namespace
func (r *DashboardAppGroupReconciler) ensureConfigMapExists(ctx context.Context, namespaceName string) error {
	logger := log.FromContext(ctx)

	configMapName := ConfigMapNamePrefix + namespaceName
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: namespaceName}, configMap)

	if err == nil {
		// ConfigMap already exists
		logger.Info("ConfigMap already exists", "namespace", namespaceName, "configmap", configMapName)
		return nil
	}

	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to check for ConfigMap: %w", err)
	}

	// ConfigMap doesn't exist, create it
	// We'll use the NamespaceReconciler's logic to generate the ConfigMap
	// For now, create a basic ConfigMap that will be populated by the NamespaceReconciler
	logger.Info("Creating ConfigMap for namespace", "namespace", namespaceName, "configmap", configMapName)

	newConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespaceName,
			Labels: map[string]string{
				ConfigMapTypeLabel: ConfigMapTypeValue,
			},
		},
		Data: map[string]string{
			"config.yaml": fmt.Sprintf(`# Dashboard configuration for namespace: %s
# This ConfigMap will be automatically populated by the operator
# Edit this file to customize how apps appear in the dashboard

apps: {}
`, namespaceName),
		},
	}

	err = r.Create(ctx, newConfigMap)
	if err != nil {
		return fmt.Errorf("failed to create ConfigMap: %w", err)
	}

	logger.Info("Successfully created ConfigMap", "namespace", namespaceName, "configmap", configMapName)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DashboardAppGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dashboardv1alpha1.DashboardAppGroup{}).
		Complete(r)
}

// Made with Bob
