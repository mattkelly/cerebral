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

// AutoScalingGroup describes a node group for autoscaling
type AutoScalingGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AutoScalingGroupSpec   `json:"spec"`
	Status AutoScalingGroupStatus `json:"status"`
}

// AutoScalingGroupSpec is the spec for a autoscaling group
type AutoScalingGroupSpec struct {
	NodeSelector   map[string]string `json:"nodeSelector"`
	Policies       []string          `json:"policies"`
	Engine         string            `json:"engine"`
	CooldownPeriod int               `json:"cooldownPeriod"`
	Suspend        bool              `json:"suspend"`
	MinNodes       int               `json:"minNodes"`
	MaxNodes       int               `json:"maxNodes"`
}

// AutoScalingGroupStatus is the status for a autoscaling group
type AutoScalingGroupStatus struct {
	LastUpdatedAt int64 `json:"LastUpdatedAt"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AutoScalingGroupList is a list of autoscaling groups.
type AutoScalingGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AutoScalingGroup `json:"items"`
}

// +genclient
// +genclient:noStatus
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AutoscalingPolicy describes a source for metrics for autoscaling
type AutoscalingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AutoscalingPolicySpec `json:"spec"`
}

// AutoscalingPolicySpec is the spec for a metrics backend
type AutoscalingPolicySpec struct {
	MetricsBackend      string              `json:"metricsBackend"`
	Metric              string              `json:"metric"`
	MetricConfiguration map[string]string   `json:"metricConfiguration"`
	Policy              PolicyConfiguration `json:"policy"`
}

type PolicyConfiguration struct {
	ScaleUp   ScaleConfiguration `json:"scaleUp"`
	ScaleDown ScaleConfiguration `json:"scaleDown"`
}

type ScaleConfiguration struct {
	Threshold float32 `json:"threshold"`
	// TODO real types for below
	ComparisonOperator string `json:"comparisonOperator"`
	AdjustmentType     string `json:"adjustmentType"`
	AdjustmentValue    string `json:"adjustmentValue"`
	Heuristic          string `json:"heuristic,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AutoscalingPolicyList is a list of AutoscalingPolicys.
type AutoscalingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AutoscalingPolicy `json:"items"`
}
