package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dashboardv1alpha1 "github.com/yamlwrangler/app-dashboard-operator/api/v1alpha1"
)

const (
	discoveryModeMerge   = "merge"
	discoveryModeReplace = "replace"
	discoveryModeNone    = "none"
)

// DashboardNamespaceConfigReconciler reconciles DashboardNamespaceConfig resources.
type DashboardNamespaceConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=dashboard.yamlwrangler.com,resources=dashboardnamespaceconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dashboard.yamlwrangler.com,resources=dashboardnamespaceconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dashboard.yamlwrangler.com,resources=dashboardnamespaceconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch

// Reconcile syncs structured namespace dashboard config into dashboard-config-<namespace>.
func (r *DashboardNamespaceConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	configCR := &dashboardv1alpha1.DashboardNamespaceConfig{}
	if err := r.Get(ctx, req.NamespacedName, configCR); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if configCR.Spec.Enabled {
		if err := ensureDashboardNamespaceLabel(ctx, r.Client, configCR.Namespace); err != nil {
			_ = r.setDashboardNamespaceConfigStatus(ctx, configCR, 0, metav1.ConditionFalse, "NamespaceLabelFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	configMapName := ConfigMapNamePrefix + configCR.Namespace
	existingConfig, configMap, err := r.loadNamespaceConfigMap(ctx, configCR.Namespace, configMapName)
	if err != nil {
		_ = r.setDashboardNamespaceConfigStatus(ctx, configCR, 0, metav1.ConditionFalse, "ConfigMapReadFailed", err.Error())
		return ctrl.Result{}, err
	}

	desired := cloneNamespaceConfig(existingConfig)
	mode := normalizeDiscoveryMode(configCR.Spec.DiscoveryMode)
	if mode != discoveryModeNone {
		discovered, err := (&NamespaceReconciler{Client: r.Client, Scheme: r.Scheme}).discoverNamespaceConfig(ctx, configCR.Namespace)
		if err != nil {
			_ = r.setDashboardNamespaceConfigStatus(ctx, configCR, 0, metav1.ConditionFalse, "DiscoveryFailed", err.Error())
			return ctrl.Result{}, err
		}

		if mode == discoveryModeReplace {
			desired = discovered
		} else {
			mergeDiscoveredApps(&desired, discovered)
		}
	}

	overlayDashboardNamespaceConfig(&desired, configCR.Spec.Apps)

	if configMap == nil || !reflect.DeepEqual(existingConfig, desired) {
		if err := upsertNamespaceConfigMap(ctx, r.Client, configMap, configCR.Namespace, configMapName, desired); err != nil {
			_ = r.setDashboardNamespaceConfigStatus(ctx, configCR, len(desired.Apps), metav1.ConditionFalse, "ConfigMapUpdateFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	logger.Info("Reconciled DashboardNamespaceConfig", "namespace", configCR.Namespace, "configmap", configMapName, "apps", len(desired.Apps))
	if err := r.setDashboardNamespaceConfigStatus(ctx, configCR, len(desired.Apps), metav1.ConditionTrue, "Reconciled", "Dashboard namespace ConfigMap is reconciled"); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DashboardNamespaceConfigReconciler) loadNamespaceConfigMap(ctx context.Context, namespace, name string) (NamespaceConfig, *corev1.ConfigMap, error) {
	return loadNamespaceConfigMap(ctx, r.Client, namespace, name)
}

// loadNamespaceConfigMap is the shared implementation used by multiple reconcilers.
func loadNamespaceConfigMap(ctx context.Context, c client.Client, namespace, name string) (NamespaceConfig, *corev1.ConfigMap, error) {
	configMap := &corev1.ConfigMap{}
	err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, configMap)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return NamespaceConfig{Apps: map[string]AppConfig{}}, nil, nil
		}
		return NamespaceConfig{}, nil, err
	}

	config := NamespaceConfig{Apps: map[string]AppConfig{}}
	if configMap.Data != nil && configMap.Data["config.yaml"] != "" {
		if err := yaml.Unmarshal([]byte(configMap.Data["config.yaml"]), &config); err != nil {
			return NamespaceConfig{}, nil, fmt.Errorf("failed to parse %s/%s config.yaml: %w", namespace, name, err)
		}
	}
	if config.Apps == nil {
		config.Apps = map[string]AppConfig{}
	}

	return config, configMap, nil
}

func (r *DashboardNamespaceConfigReconciler) setDashboardNamespaceConfigStatus(ctx context.Context, configCR *dashboardv1alpha1.DashboardNamespaceConfig, appCount int, status metav1.ConditionStatus, reason, message string) error {
	latest := &dashboardv1alpha1.DashboardNamespaceConfig{}
	if err := r.Get(ctx, types.NamespacedName{Name: configCR.Name, Namespace: configCR.Namespace}, latest); err != nil {
		return err
	}

	latest.Status.ObservedGeneration = latest.Generation
	latest.Status.ConfigMapName = ConfigMapNamePrefix + latest.Namespace
	latest.Status.AppCount = appCount
	meta.SetStatusCondition(&latest.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: latest.Generation,
		LastTransitionTime: metav1.Now(),
	})

	return r.Status().Update(ctx, latest)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DashboardNamespaceConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dashboardv1alpha1.DashboardNamespaceConfig{}).
		Complete(r)
}

func normalizeDiscoveryMode(mode string) string {
	switch strings.ToLower(mode) {
	case discoveryModeReplace:
		return discoveryModeReplace
	case discoveryModeNone:
		return discoveryModeNone
	default:
		return discoveryModeMerge
	}
}

func cloneNamespaceConfig(config NamespaceConfig) NamespaceConfig {
	clone := NamespaceConfig{Apps: map[string]AppConfig{}}
	for name, app := range config.Apps {
		appClone := app
		if len(app.CustomLinks) > 0 {
			appClone.CustomLinks = append([]CustomLinkEntry(nil), app.CustomLinks...)
		}
		clone.Apps[name] = appClone
	}
	return clone
}

func mergeDiscoveredApps(existing *NamespaceConfig, discovered NamespaceConfig) {
	if existing.Apps == nil {
		existing.Apps = map[string]AppConfig{}
	}

	for name, discoveredApp := range discovered.Apps {
		current, found := existing.Apps[name]
		if !found {
			existing.Apps[name] = discoveredApp
			continue
		}
		if current.PrimaryRoute == "" && discoveredApp.PrimaryRoute != "" {
			current.PrimaryRoute = discoveredApp.PrimaryRoute
		}
		if current.DisplayName == "" {
			current.DisplayName = discoveredApp.DisplayName
		}
		if current.Category == "" {
			current.Category = discoveredApp.Category
		}
		if current.Description == "" {
			current.Description = discoveredApp.Description
		}
		if current.GroupWith == "" {
			current.GroupWith = discoveredApp.GroupWith
		}
		existing.Apps[name] = current
	}
}

func overlayDashboardNamespaceConfig(config *NamespaceConfig, apps map[string]dashboardv1alpha1.DashboardAppConfig) {
	if config.Apps == nil {
		config.Apps = map[string]AppConfig{}
	}

	for name, app := range apps {
		current := config.Apps[name]
		if app.Enabled != nil {
			current.Enabled = *app.Enabled
		}
		if app.DisplayName != "" {
			current.DisplayName = app.DisplayName
		}
		if app.Category != "" {
			current.Category = app.Category
		}
		if app.Description != "" {
			current.Description = app.Description
		}
		if app.PrimaryRoute != "" {
			current.PrimaryRoute = app.PrimaryRoute
		}
		if app.GroupWith != "" {
			current.GroupWith = app.GroupWith
		}
		if len(app.CustomLinks) > 0 {
			current.CustomLinks = dashboardCustomLinksToConfig(app.CustomLinks)
		}
		config.Apps[name] = current
	}
}

func dashboardCustomLinksToConfig(links []dashboardv1alpha1.DashboardCustomLink) []CustomLinkEntry {
	entries := make([]CustomLinkEntry, 0, len(links))
	for _, link := range links {
		entries = append(entries, CustomLinkEntry{
			Name:        link.Name,
			URL:         link.URL,
			Route:       link.Route,
			Description: link.Description,
		})
	}
	return entries
}

func upsertNamespaceConfigMap(ctx context.Context, c client.Client, configMap *corev1.ConfigMap, namespace, name string, config NamespaceConfig) error {
	if configMap == nil {
		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					ConfigMapTypeLabel: ConfigMapTypeValue,
				},
			},
			Data: map[string]string{},
		}
		configMap.Data["config.yaml"] = renderNamespaceConfig(namespace, config)
		return c.Create(ctx, configMap)
	}

	if configMap.Labels == nil {
		configMap.Labels = map[string]string{}
	}
	configMap.Labels[ConfigMapTypeLabel] = ConfigMapTypeValue
	if configMap.Data == nil {
		configMap.Data = map[string]string{}
	}
	configMap.Data["config.yaml"] = renderNamespaceConfig(namespace, config)
	return c.Update(ctx, configMap)
}

func ensureDashboardNamespaceLabel(ctx context.Context, c client.Client, namespaceName string) error {
	namespace := &corev1.Namespace{}
	if err := c.Get(ctx, types.NamespacedName{Name: namespaceName}, namespace); err != nil {
		return fmt.Errorf("failed to get namespace: %w", err)
	}

	if namespace.Labels != nil && namespace.Labels[NamespaceEnabledLabel] == "true" {
		return nil
	}
	if namespace.Labels == nil {
		namespace.Labels = map[string]string{}
	}
	namespace.Labels[NamespaceEnabledLabel] = "true"
	return c.Update(ctx, namespace)
}
