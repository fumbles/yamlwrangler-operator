package controllers

import (
	"context"
	"fmt"
	"reflect"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dashboardv1alpha1 "github.com/yamlwrangler/app-dashboard-operator/api/v1alpha1"
)

const DashboardLinkFinalizer = "dashboard.yamlwrangler.com/dashboardlink-finalizer"

// DashboardLinkReconciler reconciles DashboardLink resources.
type DashboardLinkReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=dashboard.yamlwrangler.com,resources=dashboardlinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dashboard.yamlwrangler.com,resources=dashboardlinks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dashboard.yamlwrangler.com,resources=dashboardlinks/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch

// Reconcile adds or updates a single custom link in dashboard-config-<namespace>.
func (r *DashboardLinkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	link := &dashboardv1alpha1.DashboardLink{}
	if err := r.Get(ctx, req.NamespacedName, link); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !link.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(link, DashboardLinkFinalizer) {
			handled, err := r.deleteImportedCustomLinkConfigMap(ctx, link)
			if err != nil {
				_ = r.setDashboardLinkStatus(ctx, link, metav1.ConditionFalse, "ConfigMapUpdateFailed", err.Error())
				return ctrl.Result{}, err
			}
			if !handled {
				if err := r.removeLinkFromConfigMap(ctx, link); err != nil {
					_ = r.setDashboardLinkStatus(ctx, link, metav1.ConditionFalse, "ConfigMapUpdateFailed", err.Error())
					return ctrl.Result{}, err
				}
			}
			controllerutil.RemoveFinalizer(link, DashboardLinkFinalizer)
			return ctrl.Result{}, r.Update(ctx, link)
		}
		return ctrl.Result{}, nil
	}

	if link.Spec.App == "" || link.Spec.Name == "" || (link.Spec.URL == "" && link.Spec.Route == "") {
		err := fmt.Errorf("spec.app, spec.name, and one of spec.url or spec.route are required")
		_ = r.setDashboardLinkStatus(ctx, link, metav1.ConditionFalse, "InvalidSpec", err.Error())
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(link, DashboardLinkFinalizer) {
		controllerutil.AddFinalizer(link, DashboardLinkFinalizer)
		return ctrl.Result{}, r.Update(ctx, link)
	}

	if link.Annotations[ImportedFromCustomLinkConfigMap] != "" {
		if err := r.syncCustomLinkConfigMapFromDashboardLink(ctx, link); err != nil {
			_ = r.setDashboardLinkStatus(ctx, link, metav1.ConditionFalse, "ConfigMapUpdateFailed", err.Error())
			return ctrl.Result{}, err
		}
		logger.Info("Reconciled imported custom-link ConfigMap", "namespace", link.Namespace, "configmap", link.Annotations[ImportedFromCustomLinkConfigMap])
		if err := r.setDashboardLinkStatus(ctx, link, metav1.ConditionTrue, "Reconciled", "Dashboard link ConfigMap is reconciled"); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := ensureDashboardNamespaceLabel(ctx, r.Client, link.Namespace); err != nil {
		_ = r.setDashboardLinkStatus(ctx, link, metav1.ConditionFalse, "NamespaceLabelFailed", err.Error())
		return ctrl.Result{}, err
	}

	config, configMap, err := r.loadLinkNamespaceConfig(ctx, link.Namespace)
	if err != nil {
		_ = r.setDashboardLinkStatus(ctx, link, metav1.ConditionFalse, "ConfigMapReadFailed", err.Error())
		return ctrl.Result{}, err
	}

	if config.Apps == nil {
		config.Apps = map[string]AppConfig{}
	}
	originalConfig := cloneNamespaceConfig(config)
	appConfig, found := config.Apps[link.Spec.App]
	if !found {
		discovered, err := (&NamespaceReconciler{Client: r.Client, Scheme: r.Scheme}).discoverNamespaceConfig(ctx, link.Namespace)
		if err != nil {
			_ = r.setDashboardLinkStatus(ctx, link, metav1.ConditionFalse, "DiscoveryFailed", err.Error())
			return ctrl.Result{}, err
		}
		appConfig = discovered.Apps[link.Spec.App]
		if appConfig.DisplayName == "" {
			appConfig.Enabled = true
			appConfig.DisplayName = titleCase(link.Spec.App)
			appConfig.Category = "Services"
			appConfig.Description = titleCase(link.Spec.App)
		}
	}

	appConfig.CustomLinks = upsertCustomLink(appConfig.CustomLinks, CustomLinkEntry{
		Name:        link.Spec.Name,
		URL:         link.Spec.URL,
		Route:       link.Spec.Route,
		Description: link.Spec.Description,
	})
	config.Apps[link.Spec.App] = appConfig

	configMapName := ConfigMapNamePrefix + link.Namespace
	if configMap == nil || !reflect.DeepEqual(originalConfig, config) {
		if err := upsertNamespaceConfigMap(ctx, r.Client, configMap, link.Namespace, configMapName, config); err != nil {
			_ = r.setDashboardLinkStatus(ctx, link, metav1.ConditionFalse, "ConfigMapUpdateFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	logger.Info("Reconciled DashboardLink", "namespace", link.Namespace, "app", link.Spec.App, "link", link.Spec.Name)
	if err := r.setDashboardLinkStatus(ctx, link, metav1.ConditionTrue, "Reconciled", "Dashboard link is reconciled"); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DashboardLinkReconciler) removeLinkFromConfigMap(ctx context.Context, link *dashboardv1alpha1.DashboardLink) error {
	config, configMap, err := r.loadLinkNamespaceConfig(ctx, link.Namespace)
	if err != nil {
		return err
	}
	if configMap == nil || config.Apps == nil {
		return nil
	}

	appConfig, found := config.Apps[link.Spec.App]
	if !found || len(appConfig.CustomLinks) == 0 {
		return nil
	}

	filtered := make([]CustomLinkEntry, 0, len(appConfig.CustomLinks))
	changed := false
	for _, current := range appConfig.CustomLinks {
		if current.Name == link.Spec.Name {
			changed = true
			continue
		}
		filtered = append(filtered, current)
	}
	if !changed {
		return nil
	}

	appConfig.CustomLinks = filtered
	config.Apps[link.Spec.App] = appConfig
	return upsertNamespaceConfigMap(ctx, r.Client, configMap, link.Namespace, ConfigMapNamePrefix+link.Namespace, config)
}

func (r *DashboardLinkReconciler) loadLinkNamespaceConfig(ctx context.Context, namespace string) (NamespaceConfig, *corev1.ConfigMap, error) {
	configMapName := ConfigMapNamePrefix + namespace
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: namespace}, configMap)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return NamespaceConfig{Apps: map[string]AppConfig{}}, nil, nil
		}
		return NamespaceConfig{}, nil, err
	}

	config := NamespaceConfig{Apps: map[string]AppConfig{}}
	if configMap.Data != nil && configMap.Data["config.yaml"] != "" {
		if err := yaml.Unmarshal([]byte(configMap.Data["config.yaml"]), &config); err != nil {
			return NamespaceConfig{}, nil, fmt.Errorf("failed to parse %s/%s config.yaml: %w", namespace, configMapName, err)
		}
	}
	if config.Apps == nil {
		config.Apps = map[string]AppConfig{}
	}

	return config, configMap, nil
}

func (r *DashboardLinkReconciler) setDashboardLinkStatus(ctx context.Context, link *dashboardv1alpha1.DashboardLink, status metav1.ConditionStatus, reason, message string) error {
	latest := &dashboardv1alpha1.DashboardLink{}
	if err := r.Get(ctx, types.NamespacedName{Name: link.Name, Namespace: link.Namespace}, latest); err != nil {
		return err
	}

	latest.Status.ObservedGeneration = latest.Generation
	latest.Status.ConfigMapName = ConfigMapNamePrefix + latest.Namespace
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
func (r *DashboardLinkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dashboardv1alpha1.DashboardLink{}).
		Complete(r)
}

func upsertCustomLink(existing []CustomLinkEntry, desired CustomLinkEntry) []CustomLinkEntry {
	for i, link := range existing {
		if link.Name == desired.Name {
			existing[i] = desired
			return existing
		}
	}

	return append(existing, desired)
}
