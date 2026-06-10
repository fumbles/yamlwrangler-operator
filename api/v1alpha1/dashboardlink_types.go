package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// DashboardLinkSpec defines one managed custom link for an app.
// +kubebuilder:validation:XValidation:rule="has(self.url) || has(self.route)",message="either url or route is required"
type DashboardLinkSpec struct {
	// App is the deployment/app key in dashboard-config-<namespace>.
	App string `json:"app"`

	// Name is the display name for the link.
	Name string `json:"name"`

	// Category groups standalone custom links in the dashboard.
	// +optional
	Category string `json:"category,omitempty"`

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

// DashboardLinkStatus defines the observed link state.
type DashboardLinkStatus struct {
	// ObservedGeneration is the most recent generation reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ConfigMapName is the dashboard ConfigMap updated by this link.
	// +optional
	ConfigMapName string `json:"configMapName,omitempty"`

	// Conditions represent the latest observations.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=dl;dlinks
// +kubebuilder:printcolumn:name="App",type=string,JSONPath=`.spec.app`
// +kubebuilder:printcolumn:name="Link",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// DashboardLink is the Schema for a managed dashboard custom link.
type DashboardLink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DashboardLinkSpec   `json:"spec,omitempty"`
	Status DashboardLinkStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DashboardLinkList contains a list of DashboardLink.
type DashboardLinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DashboardLink `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DashboardLink{}, &DashboardLinkList{})
}
