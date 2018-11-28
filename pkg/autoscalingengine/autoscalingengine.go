package autoscalingengine

import (
	"fmt"

	"github.com/containership/cluster-manager/pkg/log"

	"k8s.io/apimachinery/pkg/labels"
)

// Interface specifies the functions that an Autoscaling Engine must implement
type Interface interface {
	Name() string
	SetTargetNodeCount(nodeSelectorList labels.Selector, numNodes int, heuristic string) (bool, error)
}

// AutoscalingEngine keeps track of the engines that have been registered
type AutoscalingEngine struct {
	autoscalingEngines map[string]Interface
}

// New returns a new instances of AutoscalingEngine
func New() *AutoscalingEngine {
	return &AutoscalingEngine{
		autoscalingEngines: make(map[string]Interface),
	}
}

// Register initializes an instance of an autoscaling engine
func (ae *AutoscalingEngine) Register(engine Interface) {
	if _, found := ae.autoscalingEngines[engine.Name()]; found {
		return
	}

	log.Infof("Registered Autoscaling Engine %q", engine.Name())
	ae.autoscalingEngines[engine.Name()] = engine
}

// Exists returns true if name corresponds to an already registered
// autoscaling engine.
func (ae *AutoscalingEngine) Exists(name string) bool {
	_, found := ae.autoscalingEngines[name]
	return found
}

// Get returns an instance of the autoscaling engine
func (ae *AutoscalingEngine) Get(name string) (Interface, error) {
	e, found := ae.autoscalingEngines[name]
	if !found {
		return nil, fmt.Errorf("Autoscaling Engine '%s' not found", name)
	}

	return e, nil
}
