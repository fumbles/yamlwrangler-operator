package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	routev1 "github.com/openshift/api/route/v1"
	"gopkg.in/yaml.v2"
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

const ImportedFromConfigMapAnnotation = "dashboard.yamlwrangler.com/imported-from-configmap"

// ConfigMapReconciler reconciles dashboard ConfigMaps
type ConfigMapReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=dashboard.yamlwrangler.com,resources=dashboardnamespaceconfigs;dashboardlinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch

// Reconcile handles ConfigMap reconciliation
func (r *ConfigMapReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the ConfigMap
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, req.NamespacedName, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	configMapType := configMap.Labels[ConfigMapTypeLabel]
	if configMapType == ConfigMapTypeCustomLinkValue {
		if err := r.syncDashboardLinkFromCustomLinkConfigMap(ctx, configMap); err != nil {
			logger.Error(err, "Failed to sync DashboardLink from custom-link ConfigMap")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if configMapType != ConfigMapTypeValue {
		return ctrl.Result{}, nil
	}

	logger.Info("Processing dashboard ConfigMap", "namespace", configMap.Namespace, "name", configMap.Name)

	// Parse the config
	configYAML, ok := configMap.Data["config.yaml"]
	if !ok {
		logger.Error(fmt.Errorf("config.yaml not found"), "ConfigMap missing config.yaml")
		return ctrl.Result{}, nil
	}

	var config NamespaceConfig
	err = yaml.Unmarshal([]byte(configYAML), &config)
	if err != nil {
		logger.Error(err, "Failed to parse config.yaml")
		return ctrl.Result{}, err
	}
	if config.Apps == nil {
		config.Apps = map[string]AppConfig{}
	}

	if err := r.syncOperandsFromConfigMap(ctx, configMap, config); err != nil {
		logger.Error(err, "Failed to sync dashboard operands from ConfigMap")
		return ctrl.Result{}, err
	}

	// Apply configuration to each deployment
	for deploymentName, appConfig := range config.Apps {
		err := r.applyConfigToDeployment(ctx, configMap.Namespace, deploymentName, appConfig)
		if err != nil {
			logger.Error(err, "Failed to apply config to deployment", "deployment", deploymentName)
			// Continue with other deployments even if one fails
			continue
		}

		logger.Info("Successfully applied config to deployment",
			"deployment", deploymentName,
			"enabled", appConfig.Enabled,
			"category", appConfig.Category)
	}

	logger.Info("Successfully processed ConfigMap", "namespace", configMap.Namespace, "apps", len(config.Apps))
	return ctrl.Result{}, nil
}

func (r *ConfigMapReconciler) syncOperandsFromConfigMap(ctx context.Context, configMap *corev1.ConfigMap, config NamespaceConfig) error {
	if err := r.syncDashboardNamespaceConfig(ctx, configMap, config); err != nil {
		return err
	}
	return r.syncDashboardLinks(ctx, configMap, config)
}

func (r *ConfigMapReconciler) syncDashboardNamespaceConfig(ctx context.Context, configMap *corev1.ConfigMap, config NamespaceConfig) error {
	name := configMap.Name
	desiredSpec := dashboardv1alpha1.DashboardNamespaceConfigSpec{
		Enabled:       true,
		DiscoveryMode: "None",
		Apps:          namespaceConfigToDashboardApps(config),
	}

	current := &dashboardv1alpha1.DashboardNamespaceConfig{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: configMap.Namespace}, current)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, &dashboardv1alpha1.DashboardNamespaceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:        name,
					Namespace:   configMap.Namespace,
					Labels:      importedOperandLabels(),
					Annotations: importedConfigMapAnnotations(configMap),
				},
				Spec: desiredSpec,
			})
		}
		return err
	}

	if current.Annotations[ImportedFromConfigMapAnnotation] != configMap.Name {
		return nil
	}

	changed := false
	if !reflect.DeepEqual(current.Spec, desiredSpec) {
		current.Spec = desiredSpec
		changed = true
	}
	changed = ensureImportedConfigMapMetadata(current, configMap) || changed
	if !changed {
		return nil
	}

	return r.Update(ctx, current)
}

func (r *ConfigMapReconciler) syncDashboardLinks(ctx context.Context, configMap *corev1.ConfigMap, config NamespaceConfig) error {
	desiredNames := map[string]struct{}{}

	for appName, appConfig := range config.Apps {
		for _, link := range appConfig.CustomLinks {
			if link.Name == "" || (link.URL == "" && link.Route == "") {
				continue
			}

			name := importedDashboardLinkName(appName, link.Name)
			desiredNames[name] = struct{}{}
			desiredSpec := dashboardv1alpha1.DashboardLinkSpec{
				App:         appName,
				Name:        link.Name,
				URL:         link.URL,
				Route:       link.Route,
				Description: link.Description,
			}
			if err := r.upsertImportedDashboardLink(ctx, configMap, name, desiredSpec); err != nil {
				return err
			}
		}
	}

	linkList := &dashboardv1alpha1.DashboardLinkList{}
	if err := r.List(ctx, linkList, client.InNamespace(configMap.Namespace)); err != nil {
		return err
	}
	for i := range linkList.Items {
		link := &linkList.Items[i]
		if link.Annotations[ImportedFromConfigMapAnnotation] != configMap.Name {
			continue
		}
		if _, found := desiredNames[link.Name]; found {
			continue
		}
		if err := r.Delete(ctx, link); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

func (r *ConfigMapReconciler) upsertImportedDashboardLink(ctx context.Context, configMap *corev1.ConfigMap, name string, desiredSpec dashboardv1alpha1.DashboardLinkSpec) error {
	current := &dashboardv1alpha1.DashboardLink{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: configMap.Namespace}, current)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, &dashboardv1alpha1.DashboardLink{
				ObjectMeta: metav1.ObjectMeta{
					Name:        name,
					Namespace:   configMap.Namespace,
					Labels:      importedOperandLabels(),
					Annotations: importedConfigMapAnnotations(configMap),
				},
				Spec: desiredSpec,
			})
		}
		return err
	}

	if current.Annotations[ImportedFromConfigMapAnnotation] != configMap.Name {
		return nil
	}

	changed := false
	if !reflect.DeepEqual(current.Spec, desiredSpec) {
		current.Spec = desiredSpec
		changed = true
	}
	changed = ensureImportedConfigMapMetadata(current, configMap) || changed
	if !changed {
		return nil
	}

	return r.Update(ctx, current)
}

// applyConfigToDeployment applies configuration to a single deployment
func (r *ConfigMapReconciler) applyConfigToDeployment(ctx context.Context, namespace, deploymentName string, config AppConfig) error {
	logger := log.FromContext(ctx)

	// Get the deployment
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: namespace}, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Deployment not found, skipping", "deployment", deploymentName)
			return nil
		}
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// Initialize labels and annotations if nil
	if deployment.Labels == nil {
		deployment.Labels = make(map[string]string)
	}
	if deployment.Annotations == nil {
		deployment.Annotations = make(map[string]string)
	}

	// Set the enabled label
	if config.Enabled {
		deployment.Labels[LabelEnabled] = "true"
		deployment.Annotations[AnnotationEnabled] = "true"
	} else {
		// Remove the label if disabled
		delete(deployment.Labels, LabelEnabled)
		deployment.Annotations[AnnotationEnabled] = "false"
	}

	// Set annotations
	if config.DisplayName != "" {
		deployment.Annotations[AnnotationDisplayName] = config.DisplayName
	}

	if config.Category != "" {
		deployment.Annotations[AnnotationCategory] = config.Category
	}

	if config.Description != "" {
		deployment.Annotations[AnnotationDescription] = config.Description
	}

	if config.PrimaryRoute != "" {
		deployment.Annotations[AnnotationPrimaryRoute] = config.PrimaryRoute
	}

	// Handle grouping
	if config.GroupWith != "" {
		deployment.Annotations[AnnotationAppGroup] = config.GroupWith
	} else {
		// If not grouped, use own name as app-group
		deployment.Annotations[AnnotationAppGroup] = deploymentName
	}

	// Handle custom links - resolve routes to URLs
	if len(config.CustomLinks) > 0 {
		resolvedLinks := make([]CustomLinkEntry, 0, len(config.CustomLinks))

		for _, link := range config.CustomLinks {
			resolvedLink := link

			// If route is specified, resolve it to a URL
			if link.Route != "" && link.URL == "" {
				routeURL, err := r.getRouteURL(ctx, namespace, link.Route)
				if err != nil {
					logger.Error(err, "Failed to resolve route for custom link",
						"deployment", deploymentName,
						"linkName", link.Name,
						"route", link.Route)
					// Skip this link if route resolution fails
					continue
				}
				resolvedLink.URL = routeURL
			}

			// Only add links that have a URL (either provided or resolved)
			if resolvedLink.URL != "" {
				resolvedLinks = append(resolvedLinks, resolvedLink)
			}
		}

		if len(resolvedLinks) == 0 {
			delete(deployment.Annotations, AnnotationCustomLinks)
		} else {
			linksJSON, err := json.Marshal(resolvedLinks)
			if err != nil {
				logger.Error(err, "Failed to marshal custom links", "deployment", deploymentName)
			} else {
				deployment.Annotations[AnnotationCustomLinks] = string(linksJSON)
			}
		}
	} else {
		delete(deployment.Annotations, AnnotationCustomLinks)
	}

	// Update the deployment
	err = r.Update(ctx, deployment)
	if err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	return nil
}

// getRouteURL retrieves the URL for a given route name in a namespace
func (r *ConfigMapReconciler) getRouteURL(ctx context.Context, namespace, routeName string) (string, error) {
	route := &routev1.Route{}
	err := r.Get(ctx, types.NamespacedName{Name: routeName, Namespace: namespace}, route)
	if err != nil {
		return "", fmt.Errorf("failed to get route %s: %w", routeName, err)
	}

	// Build the URL from the route
	protocol := "http"
	if route.Spec.TLS != nil {
		protocol = "https"
	}

	host := route.Spec.Host
	if host == "" {
		return "", fmt.Errorf("route %s has no host", routeName)
	}

	return fmt.Sprintf("%s://%s", protocol, host), nil
}

// SetupWithManager sets up the controller with the Manager
func (r *ConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		Complete(r)
}

// Made with Bob
