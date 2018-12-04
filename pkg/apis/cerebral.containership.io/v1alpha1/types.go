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
	Type          string                            `json:"type"`
	Configuration map[string]ConfigurationInterface `json:"configuration"`
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
	ScaleUp   string `json:"scaleUp"`
	ScaleDown string `json:"scaleDown"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AutoscalingGroupList is a list of autoscaling groups.
type AutoscalingGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AutoscalingGroup `json:"items"`
}

// +genclient
// +genclient:noStatus
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AutoscalingPolicy describes a node group for autoscaling
type AutoscalingPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec AutoscalingPolicySpec `json:"spec"`
}

// AutoscalingPolicySpec is the spec for a autoscaling group
type AutoscalingPolicySpec struct {
	MetricsBackend      string                            `json:"metricsBackend"`
	Metric              string                            `json:"metric"`
	MetricConfiguration map[string]ConfigurationInterface `json:"metricConfiguration"`
	ScalingPolicy       ScalingPolicy                     `json:"scalingPolicy"`
	PollInterval        int                               `json:"pollInterval"`
	SamplePeriod        int                               `json:"samplePeriod"`
}

// ScalingPolicy holds the policy configurations for scaling up and down
type ScalingPolicy struct {
	ScaleUp   ScalingPolicyConfiguration `json:"scaleUp"`
	ScaleDown ScalingPolicyConfiguration `json:"scaleDown"`
}

// A ScalingPolicyConfiguration defines the criterion for triggering a scale event
type ScalingPolicyConfiguration struct {
	Threshold          float64 `json:"threshold"`
	ComparisonOperator string  `json:"comparisonOperator"`
	AdjustmentType     string  `json:"adjustmentType"`
	AdjustmentValue    int     `json:"adjustmentValue"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AutoscalingPolicyList is a list of autoscaling groups.
type AutoscalingPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []AutoscalingPolicy `json:"items"`
}

// ConfigurationInterface is used for configuration objects, this allows
// the DeepCopy functions for code gen to be created
// https://github.com/kubernetes/code-generator/issues/50
type ConfigurationInterface interface {
	DeepCopyConfigurationInterface() ConfigurationInterface
}
