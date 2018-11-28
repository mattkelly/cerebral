package containership

import (
	"k8s.io/apimachinery/pkg/labels"

	"github.com/containership/cluster-manager/pkg/log"
)

// Engine returns an instance of the containership autoscaling engine
// TODO: implement the containership autoscaling engine
type Engine struct {
	name string
}

// NewAutoscalingEngine creates a new instance of the containership autoScaling
// engine
func NewAutoscalingEngine() *Engine {
	return &Engine{
		name: "containership",
	}
}

// SetTargetNodeCount takes action to scale a target node pool
func (cae *Engine) SetTargetNodeCount(nodeSelectorList labels.Selector, numNodes int, strategy string) (bool, error) {
	log.Info("Called SetTargetNodeCount")
	return true, nil
}

// Name returns the name of the engine
func (cae *Engine) Name() string {
	return cae.name
}
