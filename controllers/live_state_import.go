package controllers

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dashboardv1alpha1 "github.com/yamlwrangler/app-dashboard-operator/api/v1alpha1"
)

const (
	ImportedFromLiveStateAnnotation      = "dashboard.yamlwrangler.com/imported-from-live-state"
	ImportedFromCustomLinkConfigMap      = "dashboard.yamlwrangler.com/imported-from-custom-link-configmap"
	ConfigMapTypeCustomLinkValue         = "custom-link"
	importSourceLiveDeployments          = "deployments"
	importSourceNamespaceConfigConfigMap = "namespace-config-configmap"
	importSourceCustomLinkConfigMap      = "custom-link-configmap"
)

func (r *NamespaceReconciler) syncDashboardNamespaceConfigFromLiveState(ctx context.Context, namespaceName string) (bool, error) {
	config, found, err := r.discoverLiveDashboardNamespaceConfig(ctx, namespaceName)
	if err != nil || !found {
		return found, err
	}

	desiredSpec := dashboardv1alpha1.DashboardNamespaceConfigSpec{
		Enabled:       true,
		DiscoveryMode: "None",
		Apps:          namespaceConfigToDashboardApps(config),
	}

	name := ConfigMapNamePrefix + namespaceName
	current := &dashboardv1alpha1.DashboardNamespaceConfig{}
	err = r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespaceName}, current)
	if err != nil {
		if errors.IsNotFound(err) {
			return true, r.Create(ctx, &dashboardv1alpha1.DashboardNamespaceConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:        name,
					Namespace:   namespaceName,
					Labels:      importedOperandLabels(),
					Annotations: importedLiveStateAnnotations(),
				},
				Spec: desiredSpec,
			})
		}
		return true, err
	}

	if current.Annotations[ImportedFromLiveStateAnnotation] != importSourceLiveDeployments {
		return true, nil
	}

	changed := false
	if !reflect.DeepEqual(current.Spec, desiredSpec) {
		current.Spec = desiredSpec
		changed = true
	}
	changed = ensureImportedLiveStateMetadata(current) || changed
	if !changed {
		return true, nil
	}

	return true, r.Update(ctx, current)
}

func (r *NamespaceReconciler) discoverLiveDashboardNamespaceConfig(ctx context.Context, namespaceName string) (NamespaceConfig, bool, error) {
	deployments := &appsv1.DeploymentList{}
	if err := r.List(ctx, deployments, client.InNamespace(namespaceName), client.MatchingLabels{LabelEnabled: "true"}); err != nil {
		return NamespaceConfig{}, false, fmt.Errorf("failed to list dashboard deployments: %w", err)
	}
	if len(deployments.Items) == 0 {
		return NamespaceConfig{}, false, nil
	}

	config := NamespaceConfig{Apps: map[string]AppConfig{}}
	displayNameToDeployment := map[string]string{}
	for _, deployment := range deployments.Items {
		app := appConfigFromDeployment(deployment)
		config.Apps[deployment.Name] = app
		displayNameToDeployment[app.DisplayName] = deployment.Name
	}
	for name, app := range config.Apps {
		if normalizedGroup, found := displayNameToDeployment[app.GroupWith]; found {
			app.GroupWith = normalizedGroup
			config.Apps[name] = app
		}
	}

	return config, true, nil
}

func appConfigFromDeployment(deployment appsv1.Deployment) AppConfig {
	annotations := deployment.Annotations
	app := AppConfig{
		Enabled:      true,
		DisplayName:  valueOrDefault(annotations[AnnotationDisplayName], titleCase(deployment.Name)),
		Category:     valueOrDefault(annotations[AnnotationCategory], "Uncategorized"),
		Description:  annotations[AnnotationDescription],
		PrimaryRoute: annotations[AnnotationPrimaryRoute],
		GroupWith:    annotations[AnnotationAppGroup],
	}

	if rawLinks := annotations[AnnotationCustomLinks]; rawLinks != "" {
		var links []CustomLinkEntry
		if err := json.Unmarshal([]byte(rawLinks), &links); err == nil {
			app.CustomLinks = links
		}
	}

	return app
}

func (r *ConfigMapReconciler) syncDashboardLinkFromCustomLinkConfigMap(ctx context.Context, configMap *corev1.ConfigMap) error {
	if configMap.Data == nil || configMap.Data["url"] == "" {
		return nil
	}

	spec := dashboardv1alpha1.DashboardLinkSpec{
		App:         configMap.Name,
		Name:        valueOrDefault(configMap.Data["displayName"], configMap.Name),
		Category:    configMap.Data["category"],
		URL:         configMap.Data["url"],
		Description: configMap.Data["description"],
	}

	current := &dashboardv1alpha1.DashboardLink{}
	err := r.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, current)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, &dashboardv1alpha1.DashboardLink{
				ObjectMeta: metav1.ObjectMeta{
					Name:        configMap.Name,
					Namespace:   configMap.Namespace,
					Labels:      importedOperandLabels(),
					Annotations: importedCustomLinkConfigMapAnnotations(configMap),
				},
				Spec: spec,
			})
		}
		return err
	}

	if current.Annotations[ImportedFromCustomLinkConfigMap] != configMap.Name {
		return nil
	}

	changed := false
	if !reflect.DeepEqual(current.Spec, spec) {
		current.Spec = spec
		changed = true
	}
	changed = ensureImportedCustomLinkConfigMapMetadata(current, configMap) || changed
	if !changed {
		return nil
	}

	return r.Update(ctx, current)
}

func (r *DashboardLinkReconciler) syncCustomLinkConfigMapFromDashboardLink(ctx context.Context, link *dashboardv1alpha1.DashboardLink) error {
	configMapName := link.Annotations[ImportedFromCustomLinkConfigMap]
	if configMapName == "" {
		return nil
	}

	url := link.Spec.URL
	if url == "" && link.Spec.Route != "" {
		resolved, err := (&ConfigMapReconciler{Client: r.Client, Scheme: r.Scheme}).getRouteURL(ctx, link.Namespace, link.Spec.Route)
		if err != nil {
			return err
		}
		url = resolved
	}
	if url == "" {
		return fmt.Errorf("spec.url or resolvable spec.route is required for custom-link ConfigMap import")
	}

	desired := map[string]string{
		"displayName": link.Spec.Name,
		"url":         url,
	}
	if link.Spec.Category != "" {
		desired["category"] = link.Spec.Category
	}
	if link.Spec.Description != "" {
		desired["description"] = link.Spec.Description
	}

	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: link.Namespace}, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: link.Namespace,
					Labels: map[string]string{
						ConfigMapTypeLabel: ConfigMapTypeCustomLinkValue,
					},
				},
				Data: desired,
			})
		}
		return err
	}

	changed := false
	if configMap.Labels == nil {
		configMap.Labels = map[string]string{}
		changed = true
	}
	if configMap.Labels[ConfigMapTypeLabel] != ConfigMapTypeCustomLinkValue {
		configMap.Labels[ConfigMapTypeLabel] = ConfigMapTypeCustomLinkValue
		changed = true
	}
	if !reflect.DeepEqual(configMap.Data, desired) {
		configMap.Data = desired
		changed = true
	}
	if !changed {
		return nil
	}

	return r.Update(ctx, configMap)
}

func (r *DashboardLinkReconciler) deleteImportedCustomLinkConfigMap(ctx context.Context, link *dashboardv1alpha1.DashboardLink) (bool, error) {
	configMapName := link.Annotations[ImportedFromCustomLinkConfigMap]
	if configMapName == "" {
		return false, nil
	}

	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: link.Namespace}, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			return true, nil
		}
		return true, err
	}

	if configMap.Labels[ConfigMapTypeLabel] != ConfigMapTypeCustomLinkValue {
		return true, nil
	}

	if err := r.Delete(ctx, configMap); err != nil && !errors.IsNotFound(err) {
		return true, err
	}
	return true, nil
}

func namespaceConfigToDashboardApps(config NamespaceConfig) map[string]dashboardv1alpha1.DashboardAppConfig {
	apps := make(map[string]dashboardv1alpha1.DashboardAppConfig, len(config.Apps))
	for name, app := range config.Apps {
		enabled := app.Enabled
		apps[name] = dashboardv1alpha1.DashboardAppConfig{
			Enabled:      &enabled,
			DisplayName:  app.DisplayName,
			Category:     app.Category,
			Description:  app.Description,
			PrimaryRoute: app.PrimaryRoute,
			GroupWith:    app.GroupWith,
			CustomLinks:  customLinksToDashboardCustomLinks(app.CustomLinks),
		}
	}
	return apps
}

func dashboardAppsToNamespaceConfig(apps map[string]dashboardv1alpha1.DashboardAppConfig) NamespaceConfig {
	config := NamespaceConfig{Apps: map[string]AppConfig{}}
	for name, app := range apps {
		enabled := true
		if app.Enabled != nil {
			enabled = *app.Enabled
		}
		config.Apps[name] = AppConfig{
			Enabled:      enabled,
			DisplayName:  app.DisplayName,
			Category:     app.Category,
			Description:  app.Description,
			PrimaryRoute: app.PrimaryRoute,
			GroupWith:    app.GroupWith,
			CustomLinks:  dashboardCustomLinksToConfig(app.CustomLinks),
		}
	}
	return config
}

func customLinksToDashboardCustomLinks(links []CustomLinkEntry) []dashboardv1alpha1.DashboardCustomLink {
	if len(links) == 0 {
		return nil
	}
	converted := make([]dashboardv1alpha1.DashboardCustomLink, 0, len(links))
	for _, link := range links {
		if link.Name == "" || (link.URL == "" && link.Route == "") {
			continue
		}
		converted = append(converted, dashboardv1alpha1.DashboardCustomLink{
			Name:        link.Name,
			URL:         link.URL,
			Route:       link.Route,
			Description: link.Description,
		})
	}
	return converted
}

func importedDashboardLinkName(appName, linkName string) string {
	hash := sha1.Sum([]byte(appName + "\x00" + linkName))
	suffix := "-" + hex.EncodeToString(hash[:])[:8]
	base := dnsLabel(appName + "-" + linkName)
	if base == "" {
		base = "dashboard-link"
	}
	maxBaseLength := 63 - len(suffix)
	if len(base) > maxBaseLength {
		base = strings.Trim(base[:maxBaseLength], "-")
	}
	if base == "" {
		base = "dashboard-link"
	}
	return base + suffix
}

func dnsLabel(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	lastDash := false
	for i := 0; i < len(value); i++ {
		c := value[i]
		isAlphaNumeric := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
		if isAlphaNumeric {
			b.WriteByte(c)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func importedOperandLabels() map[string]string {
	return map[string]string{
		ConfigMapTypeLabel: ConfigMapTypeValue,
	}
}

func importedConfigMapAnnotations(configMap *corev1.ConfigMap) map[string]string {
	return map[string]string{
		ImportedFromConfigMapAnnotation: configMap.Name,
		ImportedFromLiveStateAnnotation: importSourceNamespaceConfigConfigMap,
	}
}

func importedLiveStateAnnotations() map[string]string {
	return map[string]string{
		ImportedFromLiveStateAnnotation: importSourceLiveDeployments,
	}
}

func importedCustomLinkConfigMapAnnotations(configMap *corev1.ConfigMap) map[string]string {
	return map[string]string{
		ImportedFromCustomLinkConfigMap: configMap.Name,
		ImportedFromLiveStateAnnotation: importSourceCustomLinkConfigMap,
	}
}

func ensureImportedConfigMapMetadata(obj client.Object, configMap *corev1.ConfigMap) bool {
	changed := ensureImportedMetadataBase(obj)
	annotations := obj.GetAnnotations()
	if annotations[ImportedFromConfigMapAnnotation] != configMap.Name {
		annotations[ImportedFromConfigMapAnnotation] = configMap.Name
		changed = true
	}
	if annotations[ImportedFromLiveStateAnnotation] != importSourceNamespaceConfigConfigMap {
		annotations[ImportedFromLiveStateAnnotation] = importSourceNamespaceConfigConfigMap
		changed = true
	}
	return changed
}

func ensureImportedLiveStateMetadata(obj client.Object) bool {
	changed := ensureImportedMetadataBase(obj)
	annotations := obj.GetAnnotations()
	if annotations[ImportedFromLiveStateAnnotation] != importSourceLiveDeployments {
		annotations[ImportedFromLiveStateAnnotation] = importSourceLiveDeployments
		changed = true
	}
	return changed
}

func ensureImportedCustomLinkConfigMapMetadata(obj client.Object, configMap *corev1.ConfigMap) bool {
	changed := ensureImportedMetadataBase(obj)
	annotations := obj.GetAnnotations()
	if annotations[ImportedFromCustomLinkConfigMap] != configMap.Name {
		annotations[ImportedFromCustomLinkConfigMap] = configMap.Name
		changed = true
	}
	if annotations[ImportedFromLiveStateAnnotation] != importSourceCustomLinkConfigMap {
		annotations[ImportedFromLiveStateAnnotation] = importSourceCustomLinkConfigMap
		changed = true
	}
	return changed
}

func ensureImportedMetadataBase(obj client.Object) bool {
	changed := false
	if obj.GetLabels() == nil {
		obj.SetLabels(map[string]string{})
		changed = true
	}
	labels := obj.GetLabels()
	if labels[ConfigMapTypeLabel] != ConfigMapTypeValue {
		labels[ConfigMapTypeLabel] = ConfigMapTypeValue
		changed = true
	}
	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(map[string]string{})
		changed = true
	}
	return changed
}
