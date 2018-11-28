package autoscalingengine

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/containership/cluster-manager/pkg/log"

	"k8s.io/apimachinery/pkg/labels"
)

// Factory is a function that returns a autoscalingengine.Interface.
// The config parameter provides an io.Reader handler to the factory in
// order to load specific configurations. If no configuration is provided
// the parameter is nil.
type Factory func(config io.Reader) (Interface, error)

var (
	autoscalingEngineMutex sync.Mutex
	autoscalingEngines     = make(map[string]Interface)
)

// Interface specifies the functions that an Autoscaling Engine must implement
type Interface interface {
	SetTargetNodeCount(nodeSelectorList labels.Selector, numNodes int, heuristic string) (bool, error)
}

// RegisterAutoscalingEngine initializes an instance of an autoscaling engine
func RegisterAutoscalingEngine(name, config string, autoscalingengine Factory) {
	autoscalingEngineMutex.Lock()
	defer autoscalingEngineMutex.Unlock()
	if _, found := autoscalingEngines[name]; found {
		log.Fatalf("Autoscaling Engine %q has already been registered", name)
	}

	ase, err := InitAutoscalingEngine(name, config, autoscalingengine)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof("Registered Autoscaling Engine %q", name)
	autoscalingEngines[name] = ase
}

// IsAutoscalingEngine returns true if name corresponds to an already registered
// autoscaling engine.
func IsAutoscalingEngine(name string) bool {
	_, found := autoscalingEngines[name]
	return found
}

// GetAutoscalingEngine returns an instance of the autoscaling engine
func GetAutoscalingEngine(name string, config io.Reader) (Interface, error) {
	ase, found := autoscalingEngines[name]
	if !found {
		return nil, nil
	}

	return ase, nil
}

// InitAutoscalingEngine creates an instance of the named autoscaling engine.
func InitAutoscalingEngine(name, configFilePath string, autoscalingengine Factory) (Interface, error) {
	var ase Interface
	var err error

	if name == "" {
		log.Info("No autoscaling engine specified.")
		return nil, nil
	}

	if configFilePath != "" {
		var config *os.File
		config, err = os.Open(configFilePath)
		if err != nil {
			log.Fatalf("Couldn't open autoscaling engine configuration %s: %#v",
				configFilePath, err)
		}

		defer config.Close()

		ase, err = autoscalingengine(config)
	} else {
		// Pass explicit nil so plugins can actually check for nil. See
		// "Why is my nil error value not equal to nil?" in golang.org/doc/faq.
		ase, err = autoscalingengine(nil)
	}

	if err != nil {
		return nil, fmt.Errorf("could not init autoscaling engine %q: %v", name, err)
	}
	if ase == nil {
		return nil, fmt.Errorf("unknown autoscaling engine %q", name)
	}

	return ase, nil
}
