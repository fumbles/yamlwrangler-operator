package controllers

import (
	"context"
	"fmt"
	"strings"

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
)

const (
	NamespaceEnabledLabel = "dashboard.yamlwrangler.com/enabled"
	ConfigMapNamePrefix   = "dashboard-config-"
	ConfigMapTypeLabel    = "dashboard.yamlwrangler.com/type"
	ConfigMapTypeValue    = "namespace-config"
)

// NamespaceReconciler reconciles labeled Namespaces
type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// AppConfig represents configuration for a single app
type AppConfig struct {
	Enabled      bool              `yaml:"enabled"`
	DisplayName  string            `yaml:"displayName"`
	Category     string            `yaml:"category"`
	Description  string            `yaml:"description"`
	PrimaryRoute string            `yaml:"primaryRoute,omitempty"`
	GroupWith    string            `yaml:"groupWith,omitempty"`
	CustomLinks  []CustomLinkEntry `yaml:"customLinks,omitempty"`
}

// CustomLinkEntry represents a custom link
type CustomLinkEntry struct {
	Name        string `yaml:"name" json:"name"`
	URL         string `yaml:"url,omitempty" json:"url,omitempty"`
	Route       string `yaml:"route,omitempty" json:"route,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// NamespaceConfig represents the full configuration
type NamespaceConfig struct {
	Apps map[string]AppConfig `yaml:"apps"`
}

// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch

// Reconcile handles namespace reconciliation
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Namespace
	namespace := &corev1.Namespace{}
	err := r.Get(ctx, req.NamespacedName, namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Check if namespace is labeled for dashboard
	if namespace.Labels[NamespaceEnabledLabel] != "true" {
		logger.Info("Namespace not labeled for dashboard, skipping", "namespace", namespace.Name)
		return ctrl.Result{}, nil
	}

	logger.Info("Processing labeled namespace", "namespace", namespace.Name)

	// Check if ConfigMap already exists
	configMapName := ConfigMapNamePrefix + namespace.Name
	configMap := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: namespace.Name}, configMap)

	if err != nil && errors.IsNotFound(err) {
		// ConfigMap doesn't exist, generate it
		logger.Info("Generating ConfigMap for namespace", "namespace", namespace.Name)

		newConfigMap, err := r.generateConfigMap(ctx, namespace.Name)
		if err != nil {
			logger.Error(err, "Failed to generate ConfigMap")
			return ctrl.Result{}, err
		}

		err = r.Create(ctx, newConfigMap)
		if err != nil {
			logger.Error(err, "Failed to create ConfigMap")
			return ctrl.Result{}, err
		}

		logger.Info("Successfully created ConfigMap", "namespace", namespace.Name, "configmap", configMapName)
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("ConfigMap already exists", "namespace", namespace.Name, "configmap", configMapName)
	return ctrl.Result{}, nil
}

// generateConfigMap creates a ConfigMap with auto-discovered apps
func (r *NamespaceReconciler) generateConfigMap(ctx context.Context, namespaceName string) (*corev1.ConfigMap, error) {
	logger := log.FromContext(ctx)

	// List all deployments in namespace
	deploymentList := &appsv1.DeploymentList{}
	err := r.List(ctx, deploymentList, client.InNamespace(namespaceName))
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	// List all routes in namespace
	routeList := &routev1.RouteList{}
	err = r.List(ctx, routeList, client.InNamespace(namespaceName))
	if err != nil {
		logger.Info("Failed to list routes (might not be OpenShift)", "error", err)
		routeList = &routev1.RouteList{} // Empty list if routes not available
	}

	// Generate configuration
	config := NamespaceConfig{
		Apps: make(map[string]AppConfig),
	}

	for _, deployment := range deploymentList.Items {
		name := deployment.Name

		// Detect if it's a database
		isDatabase := isDatabase(name)

		// Find matching route
		routeName := findRouteForDeployment(name, routeList.Items)

		// Detect parent app for databases
		parentApp := ""
		if isDatabase {
			parentApp = detectParentApp(name)
		}

		// Generate config for this deployment
		config.Apps[name] = AppConfig{
			Enabled:      !isDatabase, // Databases disabled by default
			DisplayName:  titleCase(name),
			Category:     guessCategory(name, isDatabase),
			Description:  generateDescription(name, isDatabase),
			PrimaryRoute: routeName,
			GroupWith:    parentApp,
		}
	}

	// Convert config to YAML
	configYAML, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// Add header comment
	header := fmt.Sprintf(`# Auto-generated dashboard configuration for namespace: %s
# Edit this file to customize how apps appear in the dashboard
# The operator will apply your changes to deployments automatically
#
# Fields:
#   enabled: true/false - Show/hide in dashboard
#   displayName: Display name in dashboard
#   category: Category for grouping (Media, Services, Development, Infrastructure)
#   description: Short description
#   primaryRoute: Route name for the main URL
#   groupWith: Parent deployment name (for databases/sidecars)
#   customLinks: Additional links (list format)
#
# Example with customLinks:
#   my-app:
#     enabled: true
#     displayName: My App
#     category: Services
#     description: My application
#     primaryRoute: my-app
#     customLinks:
#       - name: sidecar-container
#         route: my-app-sidecar        # Auto-resolved from route name
#         description: Sidecar service
#       - name: Admin Panel
#         url: https://admin.example.com  # Direct URL for external links
#         description: Admin interface

`, namespaceName)

	configWithHeader := header + string(configYAML)

	// Create ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapNamePrefix + namespaceName,
			Namespace: namespaceName,
			Labels: map[string]string{
				ConfigMapTypeLabel: ConfigMapTypeValue,
			},
		},
		Data: map[string]string{
			"config.yaml": configWithHeader,
		},
	}

	return configMap, nil
}

// isDatabase checks if a deployment name indicates it's a database
func isDatabase(name string) bool {
	lowerName := strings.ToLower(name)
	dbIndicators := []string{"postgres", "postgresql", "mysql", "mariadb", "mongodb", "redis", "-db", "-database"}

	for _, indicator := range dbIndicators {
		if strings.Contains(lowerName, indicator) {
			return true
		}
	}

	return false
}

// findRouteForDeployment finds a matching route for a deployment
func findRouteForDeployment(deploymentName string, routes []routev1.Route) string {
	// Try exact match first
	for _, route := range routes {
		if route.Name == deploymentName {
			return route.Name
		}
	}

	// Try matching with common suffixes removed
	baseName := deploymentName
	baseName = strings.TrimSuffix(baseName, "-wl")
	baseName = strings.TrimSuffix(baseName, "-deployment")

	for _, route := range routes {
		if route.Name == baseName {
			return route.Name
		}
	}

	return ""
}

// detectParentApp detects the parent app for a database
func detectParentApp(name string) string {
	// Remove common database suffixes
	parentName := name
	parentName = strings.TrimSuffix(parentName, "-postgres")
	parentName = strings.TrimSuffix(parentName, "-postgresql")
	parentName = strings.TrimSuffix(parentName, "-mysql")
	parentName = strings.TrimSuffix(parentName, "-mariadb")
	parentName = strings.TrimSuffix(parentName, "-mongodb")
	parentName = strings.TrimSuffix(parentName, "-redis")
	parentName = strings.TrimSuffix(parentName, "-db")
	parentName = strings.TrimSuffix(parentName, "-database")

	// If we removed something, return the parent name
	if parentName != name {
		return parentName
	}

	return ""
}

// guessCategory guesses the category based on deployment name
func guessCategory(name string, isDatabase bool) string {
	if isDatabase {
		return "Infrastructure"
	}

	lowerName := strings.ToLower(name)

	// Media apps
	mediaKeywords := []string{"plex", "sonarr", "radarr", "prowlarr", "sabnzbd", "nzbhydra",
		"qbittorrent", "tautulli", "jdownloader", "filebrowser"}
	for _, keyword := range mediaKeywords {
		if strings.Contains(lowerName, keyword) {
			return "Media"
		}
	}

	// Development tools
	devKeywords := []string{"code", "tools", "networking", "git", "jenkins", "gitlab"}
	for _, keyword := range devKeywords {
		if strings.Contains(lowerName, keyword) {
			return "Development"
		}
	}

	// Default to Services
	return "Services"
}

// generateDescription generates a description based on deployment name
func generateDescription(name string, isDatabase bool) string {
	if isDatabase {
		parentApp := detectParentApp(name)
		if parentApp != "" {
			return fmt.Sprintf("Database for %s", titleCase(parentApp))
		}
		return "Database"
	}

	// Generate description from name
	return titleCase(name)
}

// titleCase converts a deployment name to title case
func titleCase(name string) string {
	// Replace hyphens and underscores with spaces
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Title case each word
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}

	return strings.Join(words, " ")
}

// SetupWithManager sets up the controller with the Manager
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Complete(r)
}

// Made with Bob
