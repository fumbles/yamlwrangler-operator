package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DashboardAppGroupSpec defines the desired state of DashboardAppGroup
type DashboardAppGroupSpec struct {
	// DisplayName is the human-readable name shown in the dashboard
	DisplayName string `json:"displayName"`

	// Category groups apps in the dashboard (e.g., "Development", "Production")
	Category string `json:"category"`

	// Description provides additional context about the app group
	// +optional
	Description string `json:"description,omitempty"`

	// PrimaryRoute is the main route name for the app group
	// +optional
	PrimaryRoute string `json:"primaryRoute,omitempty"`

	// Selector defines how to find deployments to include in this group
	Selector DeploymentSelector `json:"selector"`

	// AutoLabel controls whether the operator automatically applies labels
	// +optional
	// +kubebuilder:default=true
	AutoLabel bool `json:"autoLabel,omitempty"`

	// CustomLinks are additional links to show in the dashboard
	// +optional
	CustomLinks []CustomLink `json:"customLinks,omitempty"`
}

// DeploymentSelector defines how to select deployments
type DeploymentSelector struct {
	// MatchPattern is a regex pattern to match deployment names
	// +optional
	MatchPattern string `json:"matchPattern,omitempty"`

	// MatchNames is an explicit list of deployment names
	// +optional
	MatchNames []string `json:"matchNames,omitempty"`

	// MatchLabels selects deployments by label selector
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// CustomLink represents an additional link for the app
type CustomLink struct {
	// Name is the display text for the link
	Name string `json:"name"`

	// URL is the link destination
	URL string `json:"url"`

	// Icon is an optional icon name from PatternFly
	// +optional
	Icon string `json:"icon,omitempty"`
}

// DashboardAppGroupStatus defines the observed state of DashboardAppGroup
type DashboardAppGroupStatus struct {
	// MatchedDeployments lists the deployments currently matched by the selector
	// +optional
	MatchedDeployments []string `json:"matchedDeployments,omitempty"`

	// LastUpdated is the timestamp of the last reconciliation
	// +optional
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`

	// Conditions represent the latest available observations of the AppGroup's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=dag;dags
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Category",type=string,JSONPath=`.spec.category`
// +kubebuilder:printcolumn:name="Deployments",type=integer,JSONPath=`.status.matchedDeployments[*]`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// DashboardAppGroup is the Schema for the dashboardappgroups API
type DashboardAppGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DashboardAppGroupSpec   `json:"spec,omitempty"`
	Status DashboardAppGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DashboardAppGroupList contains a list of DashboardAppGroup
type DashboardAppGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DashboardAppGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DashboardAppGroup{}, &DashboardAppGroupList{})
}

// Made with Bob
