package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// DashboardNamespaceConfigSpec defines dashboard configuration for one namespace.
type DashboardNamespaceConfigSpec struct {
	// Enabled labels the namespace for dashboard discovery when true.
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// DiscoveryMode controls how discovered deployments are merged into the generated ConfigMap.
	// Supported values are Merge, Replace, and None. Empty defaults to Merge.
	// +optional
	DiscoveryMode string `json:"discoveryMode,omitempty"`

	// Apps defines app-level dashboard metadata keyed by deployment name.
	// +optional
	Apps map[string]DashboardAppConfig `json:"apps,omitempty"`
}

// DashboardAppConfig defines dashboard metadata for a deployment.
type DashboardAppConfig struct {
	// Enabled controls whether the app is shown in the dashboard.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// DisplayName is the human-readable dashboard name.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Category groups the app in the dashboard.
	// +optional
	Category string `json:"category,omitempty"`

	// Description is shown on the dashboard card.
	// +optional
	Description string `json:"description,omitempty"`

	// PrimaryRoute is the route name used as the primary app link.
	// +optional
	PrimaryRoute string `json:"primaryRoute,omitempty"`

	// GroupWith groups this deployment under another app.
	// +optional
	GroupWith string `json:"groupWith,omitempty"`

	// CustomLinks are additional links for this app.
	// +optional
	CustomLinks []DashboardCustomLink `json:"customLinks,omitempty"`
}

// DashboardCustomLink defines an additional dashboard link.
// +kubebuilder:validation:XValidation:rule="has(self.url) || has(self.route)",message="either url or route is required"
type DashboardCustomLink struct {
	// Name is the display name for the link.
	Name string `json:"name"`

	// URL is an absolute external URL. Use either URL or Route.
	// +optional
	URL string `json:"url,omitempty"`

	// Route is an OpenShift Route name in the same namespace. The operator resolves it to a URL.
	// +optional
	Route string `json:"route,omitempty"`

	// Description provides link context.
	// +optional
	Description string `json:"description,omitempty"`
}

// DashboardNamespaceConfigStatus defines the observed namespace config state.
type DashboardNamespaceConfigStatus struct {
	// ObservedGeneration is the most recent generation reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ConfigMapName is the generated dashboard ConfigMap name.
	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`

	// AppCount is the number of apps in the generated ConfigMap.
	// +optional
	AppCount int `json:"appCount,omitempty"`

	// Conditions represent the latest observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=dnc;dncs
// +kubebuilder:printcolumn:name="ConfigMap",type=string,JSONPath=`.status.configMapName`
// +kubebuilder:printcolumn:name="Apps",type=integer,JSONPath=`.status.appCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// DashboardNamespaceConfig is the Schema for namespace dashboard ConfigMap management.
type DashboardNamespaceConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DashboardNamespaceConfigSpec   `json:"spec,omitempty"`
	Status DashboardNamespaceConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DashboardNamespaceConfigList contains a list of DashboardNamespaceConfig.
type DashboardNamespaceConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DashboardNamespaceConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DashboardNamespaceConfig{}, &DashboardNamespaceConfigList{})
}
