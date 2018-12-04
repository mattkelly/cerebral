package autoscalingengine

// AutoscalingEngine specifies the functions that an AutoscalingEngine must implement
type AutoscalingEngine interface {
	Name() string
	SetTargetNodeCount(nodeSelector map[string]string, numNodes int, strategy string) (bool, error)
}
