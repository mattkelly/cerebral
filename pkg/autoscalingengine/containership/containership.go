package containership

import (
	"io"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/containership/cluster-manager/pkg/log"

	autoscalingengine "github.com/containership/cerebral/pkg/autoscalingengine"
)

// TODO implement the containership autoscaling engine

type containershipAutoScalingEngine struct{}

// NewAutoscalingEngine creates a new instances of the containership autoScaling
// engine
func NewAutoscalingEngine() (autoscalingengine.Interface, error) {
	return &containershipAutoScalingEngine{}, nil
}

func init() {
	// TODO: should this be a flag, or an env var?
	pathToConfig := ""
	autoscalingengine.RegisterAutoscalingEngine("containership", pathToConfig, func(io.Reader) (autoscalingengine.Interface, error) {
		return NewAutoscalingEngine()
	})
}

func (ase containershipAutoScalingEngine) SetTargetNodeCount(nodeSelectorList labels.Selector, numNodes int, heuristic string) (bool, error) {
	log.Info("Called SetTargetNodeCount")
	return true, nil
}
