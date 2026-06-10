package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// AppDashboardSpec defines a dashboard console plugin installation.
type AppDashboardSpec struct {
	// Namespace is where the console plugin workload is installed.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// PluginName is the ConsolePlugin name and workload base name.
	// +optional
	PluginName string `json:"pluginName,omitempty"`

	// DisplayName is shown in the OpenShift console plugin list.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Image is the console plugin image to deploy.
	// +optional
	Image string `json:"image,omitempty"`

	// ImagePullPolicy controls the console plugin deployment image pull policy.
	// +optional
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`

	// Replicas is the desired number of console plugin pods.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Port is the HTTPS port served by the plugin pod and service.
	// +optional
	Port int32 `json:"port,omitempty"`

	// BasePath is the ConsolePlugin backend base path.
	// +optional
	BasePath string `json:"basePath,omitempty"`

	// EnableConsolePlugin adds this plugin to the cluster Console operator spec.plugins list.
	// +optional
	EnableConsolePlugin *bool `json:"enableConsolePlugin,omitempty"`

	// ConsoleLink customizes the application menu ConsoleLink to the dashboard route.
	// When omitted, the operator creates a default ConsoleLink.
	// +optional
	ConsoleLink *AppDashboardConsoleLinkSpec `json:"consoleLink,omitempty"`
}

// AppDashboardConsoleLinkSpec defines an OpenShift ApplicationMenu ConsoleLink.
type AppDashboardConsoleLinkSpec struct {
	// Enabled controls whether the operator creates a ConsoleLink. It defaults to true when omitted.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Name is the ConsoleLink resource name.
	// +optional
	Name string `json:"name,omitempty"`

	// Text is the menu text.
	// +optional
	Text string `json:"text,omitempty"`

	// Href is the ConsoleLink target. It may be an absolute https URL or a console-relative path.
	// +optional
	Href string `json:"href,omitempty"`

	// Section is the application menu section.
	// +optional
	Section string `json:"section,omitempty"`

	// ImageURL is the optional application menu icon URL.
	// +optional
	ImageURL string `json:"imageURL,omitempty"`
}

// AppDashboardStatus defines the observed state of an AppDashboard install.
type AppDashboardStatus struct {
	// ObservedGeneration is the most recent generation reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Namespace is the resolved install namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// PluginName is the resolved console plugin name.
	// +optional
	PluginName string `json:"pluginName,omitempty"`

	// Conditions represent the latest observations for the dashboard install.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=ad;ads
// +kubebuilder:printcolumn:name="Plugin",type=string,JSONPath=`.status.pluginName`
// +kubebuilder:printcolumn:name="Namespace",type=string,JSONPath=`.status.namespace`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// AppDashboard is the Schema for a dashboard console plugin installation.
type AppDashboard struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppDashboardSpec   `json:"spec,omitempty"`
	Status AppDashboardStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AppDashboardList contains a list of AppDashboard.
type AppDashboardList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppDashboard `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AppDashboard{}, &AppDashboardList{})
}
