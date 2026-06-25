package controllers

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dashboardv1alpha1 "github.com/yamlwrangler/app-dashboard-operator/api/v1alpha1"
)

const (
	defaultDashboardNamespace       = "app-dashboard"
	defaultDashboardPluginName      = "app-dashboard"
	defaultDashboardDisplayName     = "Yamlwrangler App Dashboard"
	defaultDashboardImage           = "fumbles/yamlwrangler-dashboard:v1.0.0"
	defaultDashboardImagePullPolicy = string(corev1.PullIfNotPresent)
	defaultDashboardPort            = int32(9443)
	defaultDashboardBasePath        = "/"
	defaultConsoleLinkName          = "app-dashboard-link"
	defaultConsoleLinkText          = "App Dashboard"
	defaultConsoleLinkHref          = "/app-dashboard"
	defaultConsoleLinkSection       = "App Dashboard"
	defaultConsoleLinkImageURL      = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI5NiIgaGVpZ2h0PSI5NiIgdmlld0JveD0iMCAwIDk2IDk2IiBmaWxsPSJub25lIj4KICA8cmVjdCB4PSI0IiB5PSI0IiB3aWR0aD0iODgiIGhlaWdodD0iODgiIHJ4PSIyNCIgZmlsbD0iIzEyMDkxMiIvPgogIDxyZWN0IHg9IjQuNzUiIHk9IjQuNzUiIHdpZHRoPSI4Ni41IiBoZWlnaHQ9Ijg2LjUiIHJ4PSIyMy4yNSIgc3Ryb2tlPSIjN2YxZDJkIiBzdHJva2Utd2lkdGg9IjEuNSIvPgogIDxnIHRyYW5zZm9ybT0idHJhbnNsYXRlKDI0IDI0KSBzY2FsZSgyKSIgc3Ryb2tlPSIjZmI3MTg1IiBzdHJva2Utd2lkdGg9IjIuNCIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIiBzdHJva2UtbGluZWpvaW49InJvdW5kIj4KICAgIDxwYXRoIGQ9Ik0yLjk3IDEyLjkyQTIgMiAwIDAgMCAyIDE0LjYzdjMuMjRhMiAyIDAgMCAwIC45NyAxLjcxbDMgMS44YTIgMiAwIDAgMCAyLjA2IDBMMTIgMTl2LTUuNWwtNS0zLTQuMDMgMi40MloiLz4KICAgIDxwYXRoIGQ9Im03IDE2LjUtNC43NC0yLjg1Ii8+CiAgICA8cGF0aCBkPSJtNyAxNi41IDUtMyIvPgogICAgPHBhdGggZD0iTTcgMTYuNXY1LjE3Ii8+CiAgICA8cGF0aCBkPSJNMTIgMTMuNVYxOWwzLjk3IDIuMzhhMiAyIDAgMCAwIDIuMDYgMGwzLTEuOGEyIDIgMCAwIDAgLjk3LTEuNzF2LTMuMjRhMiAyIDAgMCAwLS45Ny0xLjcxTDE3IDEwLjVsLTUgM1oiLz4KICAgIDxwYXRoIGQ9Im0xNyAxNi41LTUtMyIvPgogICAgPHBhdGggZD0ibTE3IDE2LjUgNC43NC0yLjg1Ii8+CiAgICA8cGF0aCBkPSJNMTcgMTYuNXY1LjE3Ii8+CiAgICA8cGF0aCBkPSJNNy45NyA0LjQyQTIgMiAwIDAgMCA3IDYuMTN2NC4zN2w1IDMgNS0zVjYuMTNhMiAyIDAgMCAwLS45Ny0xLjcxbC0zLTEuOGEyIDIgMCAwIDAtMi4wNiAwbC0zIDEuOFoiLz4KICAgIDxwYXRoIGQ9Ik0xMiA4IDcuMjYgNS4xNSIvPgogICAgPHBhdGggZD0ibTEyIDggNC43NC0yLjg1Ii8+CiAgICA8cGF0aCBkPSJNMTIgMTMuNVY4Ii8+CiAgPC9nPgo8L3N2Zz4K"
)

var (
	consolePluginGVK = schema.GroupVersionKind{
		Group:   "console.openshift.io",
		Version: "v1",
		Kind:    "ConsolePlugin",
	}
	consoleLinkGVK = schema.GroupVersionKind{
		Group:   "console.openshift.io",
		Version: "v1",
		Kind:    "ConsoleLink",
	}
	consoleOperatorGVK = schema.GroupVersionKind{
		Group:   "operator.openshift.io",
		Version: "v1",
		Kind:    "Console",
	}
	routeGVK = schema.GroupVersionKind{
		Group:   "route.openshift.io",
		Version: "v1",
		Kind:    "Route",
	}
)

// AppDashboardReconciler reconciles AppDashboard install resources.
type AppDashboardReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type resolvedAppDashboard struct {
	Namespace           string
	PluginName          string
	DisplayName         string
	Image               string
	ImagePullPolicy     corev1.PullPolicy
	Replicas            int32
	Port                int32
	BasePath            string
	EnableConsolePlugin bool
	ConsoleLink         *dashboardv1alpha1.AppDashboardConsoleLinkSpec
}

// +kubebuilder:rbac:groups=dashboard.yamlwrangler.com,resources=appdashboards,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dashboard.yamlwrangler.com,resources=appdashboards/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dashboard.yamlwrangler.com,resources=appdashboards/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts;services;configmaps,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=console.openshift.io,resources=consoleplugins;consolelinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.openshift.io,resources=consoles,verbs=get;list;watch;update;patch

func (r *AppDashboardReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	dashboard := &dashboardv1alpha1.AppDashboard{}
	if err := r.Get(ctx, req.NamespacedName, dashboard); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	cfg := resolveAppDashboard(dashboard)
	steps := []func(context.Context, *dashboardv1alpha1.AppDashboard, resolvedAppDashboard) error{
		r.reconcileNamespace,
		r.reconcileServiceAccount,
		r.reconcileNginxConfig,
		r.reconcileService,
		r.reconcileDeployment,
		r.reconcileConsolePlugin,
		r.reconcileConsoleLink,
	}

	for _, step := range steps {
		if err := step(ctx, dashboard, cfg); err != nil {
			logger.Error(err, "Failed to reconcile AppDashboard")
			_ = r.setStatus(ctx, dashboard, cfg, metav1.ConditionFalse, "ReconcileFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	if cfg.EnableConsolePlugin {
		if err := r.enableConsolePlugin(ctx, cfg.PluginName); err != nil {
			logger.Error(err, "Failed to enable console plugin")
			_ = r.setStatus(ctx, dashboard, cfg, metav1.ConditionFalse, "ConsoleEnableFailed", err.Error())
			return ctrl.Result{}, err
		}
	}

	if err := r.setStatus(ctx, dashboard, cfg, metav1.ConditionTrue, "Reconciled", "Dashboard console plugin install is reconciled"); err != nil {
		logger.Error(err, "Failed to update AppDashboard status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func resolveAppDashboard(dashboard *dashboardv1alpha1.AppDashboard) resolvedAppDashboard {
	cfg := resolvedAppDashboard{
		Namespace:           valueOrDefault(dashboard.Spec.Namespace, defaultDashboardNamespace),
		PluginName:          valueOrDefault(dashboard.Spec.PluginName, defaultDashboardPluginName),
		DisplayName:         valueOrDefault(dashboard.Spec.DisplayName, defaultDashboardDisplayName),
		Image:               valueOrDefault(dashboard.Spec.Image, defaultDashboardImage),
		ImagePullPolicy:     corev1.PullPolicy(valueOrDefault(dashboard.Spec.ImagePullPolicy, defaultDashboardImagePullPolicy)),
		Replicas:            2,
		Port:                valueOrDefaultInt32(dashboard.Spec.Port, defaultDashboardPort),
		BasePath:            valueOrDefault(dashboard.Spec.BasePath, defaultDashboardBasePath),
		EnableConsolePlugin: true,
		ConsoleLink:         dashboard.Spec.ConsoleLink,
	}

	if dashboard.Spec.Replicas != nil {
		cfg.Replicas = *dashboard.Spec.Replicas
	}
	if dashboard.Spec.EnableConsolePlugin != nil {
		cfg.EnableConsolePlugin = *dashboard.Spec.EnableConsolePlugin
	}

	return cfg
}

func (r *AppDashboardReconciler) reconcileNamespace(ctx context.Context, _ *dashboardv1alpha1.AppDashboard, cfg resolvedAppDashboard) error {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: cfg.Namespace}}
	_, err := controllerutil.CreateOrPatch(ctx, r.Client, namespace, func() error {
		addLabels(namespace, dashboardInstallLabels(cfg))
		return nil
	})
	return err
}

func (r *AppDashboardReconciler) reconcileServiceAccount(ctx context.Context, _ *dashboardv1alpha1.AppDashboard, cfg resolvedAppDashboard) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: cfg.PluginName, Namespace: cfg.Namespace},
	}
	_, err := controllerutil.CreateOrPatch(ctx, r.Client, serviceAccount, func() error {
		addLabels(serviceAccount, dashboardInstallLabels(cfg))
		return nil
	})
	return err
}

func (r *AppDashboardReconciler) reconcileNginxConfig(ctx context.Context, _ *dashboardv1alpha1.AppDashboard, cfg resolvedAppDashboard) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: cfg.PluginName, Namespace: cfg.Namespace},
	}
	_, err := controllerutil.CreateOrPatch(ctx, r.Client, configMap, func() error {
		addLabels(configMap, dashboardInstallLabels(cfg))
		configMap.Data = map[string]string{
			"nginx.conf": dashboardNginxConfig(cfg.Port),
		}
		return nil
	})
	return err
}

func (r *AppDashboardReconciler) reconcileService(ctx context.Context, _ *dashboardv1alpha1.AppDashboard, cfg resolvedAppDashboard) error {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: cfg.PluginName, Namespace: cfg.Namespace},
	}
	_, err := controllerutil.CreateOrPatch(ctx, r.Client, service, func() error {
		addLabels(service, dashboardInstallLabels(cfg))
		if service.Annotations == nil {
			service.Annotations = map[string]string{}
		}
		service.Annotations["service.alpha.openshift.io/serving-cert-secret-name"] = dashboardCertificateSecretName(cfg)
		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.Selector = dashboardSelectorLabels(cfg)
		service.Spec.Ports = []corev1.ServicePort{{
			Name:       fmt.Sprintf("%d-tcp", cfg.Port),
			Protocol:   corev1.ProtocolTCP,
			Port:       cfg.Port,
			TargetPort: intstr.FromInt32(cfg.Port),
		}}
		return nil
	})
	return err
}

func (r *AppDashboardReconciler) reconcileDeployment(ctx context.Context, _ *dashboardv1alpha1.AppDashboard, cfg resolvedAppDashboard) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: cfg.PluginName, Namespace: cfg.Namespace},
	}
	_, err := controllerutil.CreateOrPatch(ctx, r.Client, deployment, func() error {
		labels := dashboardInstallLabels(cfg)
		selectorLabels := dashboardSelectorLabels(cfg)
		addLabels(deployment, labels)
		deployment.Spec.Replicas = &cfg.Replicas
		deployment.Spec.Selector = &metav1.LabelSelector{MatchLabels: selectorLabels}
		templateLabels := maps.Clone(labels)
		maps.Copy(templateLabels, selectorLabels)
		deployment.Spec.Template.Labels = templateLabels
		deployment.Spec.Template.Spec.ServiceAccountName = cfg.PluginName
		deployment.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyAlways
		deployment.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			RunAsNonRoot: boolPtr(true),
			SeccompProfile: &corev1.SeccompProfile{
				Type: corev1.SeccompProfileTypeRuntimeDefault,
			},
		}
		deployment.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:            cfg.PluginName,
			Image:           cfg.Image,
			ImagePullPolicy: cfg.ImagePullPolicy,
			Ports: []corev1.ContainerPort{{
				ContainerPort: cfg.Port,
				Protocol:      corev1.ProtocolTCP,
			}},
			SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: boolPtr(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10m"),
					corev1.ResourceMemory: resource.MustParse("50Mi"),
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: dashboardCertificateSecretName(cfg), ReadOnly: true, MountPath: "/var/cert"},
				{Name: "nginx-conf", ReadOnly: true, MountPath: "/etc/nginx/nginx.conf", SubPath: "nginx.conf"},
			},
		}}
		deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: dashboardCertificateSecretName(cfg),
				VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
					SecretName:  dashboardCertificateSecretName(cfg),
					DefaultMode: int32Ptr(420),
				}},
			},
			{
				Name: "nginx-conf",
				VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cfg.PluginName},
					DefaultMode:          int32Ptr(420),
				}},
			},
		}
		return nil
	})
	return err
}

func (r *AppDashboardReconciler) reconcileConsolePlugin(ctx context.Context, _ *dashboardv1alpha1.AppDashboard, cfg resolvedAppDashboard) error {
	plugin := &unstructured.Unstructured{}
	plugin.SetGroupVersionKind(consolePluginGVK)
	plugin.SetName(cfg.PluginName)

	_, err := controllerutil.CreateOrPatch(ctx, r.Client, plugin, func() error {
		addLabels(plugin, dashboardInstallLabels(cfg))
		plugin.Object["spec"] = map[string]interface{}{
			"displayName": cfg.DisplayName,
			"i18n": map[string]interface{}{
				"loadType": "Preload",
			},
			"backend": map[string]interface{}{
				"type": "Service",
				"service": map[string]interface{}{
					"name":      cfg.PluginName,
					"namespace": cfg.Namespace,
					"port":      cfg.Port,
					"basePath":  cfg.BasePath,
				},
			},
		}
		return nil
	})
	return err
}

func (r *AppDashboardReconciler) reconcileConsoleLink(ctx context.Context, _ *dashboardv1alpha1.AppDashboard, cfg resolvedAppDashboard) error {
	if !consoleLinkEnabled(cfg.ConsoleLink) {
		return nil
	}
	href, err := r.resolveConsoleLinkHref(ctx, cfg.ConsoleLink)
	if err != nil {
		return err
	}

	linkName := defaultConsoleLinkName
	linkText := defaultConsoleLinkText
	linkSection := defaultConsoleLinkSection
	linkImageURL := defaultConsoleLinkImageURL
	if cfg.ConsoleLink != nil {
		linkName = valueOrDefault(cfg.ConsoleLink.Name, linkName)
		linkText = valueOrDefault(cfg.ConsoleLink.Text, linkText)
		linkSection = valueOrDefault(cfg.ConsoleLink.Section, linkSection)
		linkImageURL = valueOrDefault(cfg.ConsoleLink.ImageURL, linkImageURL)
	}

	consoleLink := &unstructured.Unstructured{}
	consoleLink.SetGroupVersionKind(consoleLinkGVK)
	consoleLink.SetName(linkName)

	_, err = controllerutil.CreateOrPatch(ctx, r.Client, consoleLink, func() error {
		addLabels(consoleLink, dashboardInstallLabels(cfg))
		consoleLink.Object["spec"] = map[string]interface{}{
			"location": "ApplicationMenu",
			"text":     linkText,
			"href":     href,
			"applicationMenu": map[string]interface{}{
				"section":  linkSection,
				"imageURL": linkImageURL,
			},
		}
		return nil
	})
	return err
}

func (r *AppDashboardReconciler) resolveConsoleLinkHref(ctx context.Context, link *dashboardv1alpha1.AppDashboardConsoleLinkSpec) (string, error) {
	href := defaultConsoleLinkHref
	if link != nil {
		href = valueOrDefault(link.Href, href)
	}
	if strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "mailto:") {
		return href, nil
	}
	if !strings.HasPrefix(href, "/") {
		href = "/" + href
	}

	consoleRoute := &unstructured.Unstructured{}
	consoleRoute.SetGroupVersionKind(routeGVK)
	if err := r.Get(ctx, types.NamespacedName{Name: "console", Namespace: "openshift-console"}, consoleRoute); err != nil {
		return "", err
	}
	host, _, err := unstructured.NestedString(consoleRoute.Object, "spec", "host")
	if err != nil {
		return "", err
	}
	if host == "" {
		return "", fmt.Errorf("openshift-console/console route has no spec.host")
	}
	return "https://" + host + href, nil
}

func (r *AppDashboardReconciler) enableConsolePlugin(ctx context.Context, pluginName string) error {
	console := &unstructured.Unstructured{}
	console.SetGroupVersionKind(consoleOperatorGVK)
	if err := r.Get(ctx, types.NamespacedName{Name: "cluster"}, console); err != nil {
		return err
	}

	existing, _, err := unstructured.NestedStringSlice(console.Object, "spec", "plugins")
	if err != nil {
		return err
	}
	for _, plugin := range existing {
		if plugin == pluginName {
			return nil
		}
	}

	plugins := append(existing, pluginName)
	sort.Strings(plugins)
	if err := unstructured.SetNestedStringSlice(console.Object, plugins, "spec", "plugins"); err != nil {
		return err
	}

	return r.Update(ctx, console)
}

func (r *AppDashboardReconciler) setStatus(ctx context.Context, dashboard *dashboardv1alpha1.AppDashboard, cfg resolvedAppDashboard, status metav1.ConditionStatus, reason, message string) error {
	latest := &dashboardv1alpha1.AppDashboard{}
	if err := r.Get(ctx, types.NamespacedName{Name: dashboard.Name}, latest); err != nil {
		return err
	}

	latest.Status.ObservedGeneration = latest.Generation
	latest.Status.Namespace = cfg.Namespace
	latest.Status.PluginName = cfg.PluginName
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

func (r *AppDashboardReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dashboardv1alpha1.AppDashboard{}).
		Complete(r)
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func valueOrDefaultInt32(value, fallback int32) int32 {
	if value == 0 {
		return fallback
	}
	return value
}

func dashboardInstallLabels(cfg resolvedAppDashboard) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":       cfg.PluginName,
		"app.kubernetes.io/part-of":    "yamlwrangler-dashboard",
		"app.kubernetes.io/managed-by": "app-dashboard-operator",
	}
}

func dashboardSelectorLabels(cfg resolvedAppDashboard) map[string]string {
	return map[string]string{
		"app": cfg.PluginName,
	}
}

func dashboardCertificateSecretName(cfg resolvedAppDashboard) string {
	return cfg.PluginName + "-cert"
}

func consoleLinkEnabled(link *dashboardv1alpha1.AppDashboardConsoleLinkSpec) bool {
	return link == nil || link.Enabled == nil || *link.Enabled
}

func boolPtr(value bool) *bool { return &value }

func int32Ptr(value int32) *int32 { return &value }

// addLabels merges extra labels into obj without clobbering unrelated existing labels.
func addLabels(obj client.Object, extra map[string]string) {
	existing := obj.GetLabels()
	if existing == nil {
		existing = map[string]string{}
	}
	for k, v := range extra {
		existing[k] = v
	}
	obj.SetLabels(existing)
}

func dashboardNginxConfig(port int32) string {
	return fmt.Sprintf(`error_log /dev/stdout info;
events {}
http {
  access_log         /dev/stdout;
  default_type       application/octet-stream;
  keepalive_timeout  65;

  types {
    text/html                             html htm shtml;
    text/css                              css;
    text/xml                              xml;
    image/gif                             gif;
    image/jpeg                            jpeg jpg;
    application/javascript                js;
    application/json                      json;
    application/woff                      woff;
    application/woff2                     woff2;
    font/ttf                              ttf;
    font/otf                              otf;
    image/svg+xml                         svg svgz;
    image/png                             png;
    image/x-icon                          ico;
  }

  server {
    listen              %d ssl;
    listen              [::]:%d ssl;
    ssl_certificate     /var/cert/tls.crt;
    ssl_certificate_key /var/cert/tls.key;
    root                /usr/share/nginx/html;

    location ~ \.js$ {
      add_header Content-Type application/javascript;
      add_header Cache-Control "public, max-age=31536000, immutable";
    }

    location ~ \.json$ {
      add_header Content-Type application/json;
    }

    location = /plugin-manifest.json {
      add_header Content-Type application/json;
      add_header Cache-Control "no-cache";
    }
  }
}
`, port, port)
}
