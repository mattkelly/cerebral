package autoscaling

// Engine specifies the functions that an Engine must implement
type Engine interface {
	Name() string
	SetTargetNodeCount(nodeSelector map[string]string, numNodes int, strategy string) (bool, error)
}
