package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MetricsBackend describes a source for metrics for autoscaling
type MetricsBackend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MetricsBackendSpec `json:"spec"`
}

// MetricsBackendSpec is the spec for a metrics backend
type MetricsBackendSpec struct {
	Address string `json:"address"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MetricsBackendList is a list of MetricsBackends.
type MetricsBackendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []MetricsBackend `json:"items"`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AutoscalingGroup describes a node group for autoscaling
type AutoscalingGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AutoscalingGroupSpec   `json:"spec"`
	Status AutoscalingGroupStatus `json:"status"`
}

// AutoscalingGroupSpec is the spec for a autoscaling group
type AutoscalingGroupSpec struct {
	NodeSelector    map[string]string `json:"nodeSelector"`
	Policies        []string          `json:"policies"`
	Engine          string            `json:"engine"`
	CooldownPeriod  int               `json:"cooldownPeriod"`
	Suspended       bool              `json:"suspend"`
	MinNodes        int               `json:"minNodes"`
	MaxNodes        int               `json:"maxNodes"`
	ScalingStrategy ScalingStrategy   `json:"scalingStrategy"`
}

// AutoscalingGroupStatus is the status for a autoscaling group
type AutoscalingGroupStatus struct {
	// LastUpdatedAt is a Unix time, time.Time is not a valid type for code gen
	LastUpdatedAt int64 `json:"lastUpdatedAt"`
}

// ScalingStrategy defines the strategy that should be used when scaling up and down
type ScalingStrategy struct {
	Up   string `json:"up"`
	Down string `json:"down"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AutoscalingGroupList is a list of autoscaling groups.
type AutoscalingGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AutoscalingGroup `json:"items"`
}
