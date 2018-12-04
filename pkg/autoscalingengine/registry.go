package autoscalingengine

import (
	"github.com/containership/cluster-manager/pkg/log"

	"github.com/pkg/errors"
)

// RegistryInterface is an interface for an AutoscalingEngin. This
// registry is not thread safe.
type RegistryInterface interface {
	Put(engine AutoscalingEngine)
	Get(name string) (AutoscalingEngine, error)
	IsRegistered(name string) bool
}

type registry struct {
	items map[string]AutoscalingEngine
}

var reg = registry{
	items: make(map[string]AutoscalingEngine),
}

// Registry returns the registry
func Registry() RegistryInterface {
	return &reg
}

// Put registers an instance of an AutoscalingEngine to the registry, using the
// AutoscalingEngine's name. If the AutoscalingEngine already exists it will be
// overwritten
func (r *registry) Put(engine AutoscalingEngine) {
	log.Infof("Registered Autoscaling Engine %q", engine.Name())
	r.items[engine.Name()] = engine
}

// IsRegistered returns true if the name corresponds to an engine that has been
// registered
func (r *registry) IsRegistered(name string) bool {
	_, found := r.items[name]
	return found
}

// Get returns the AutoscalingEngine that is registered to the name
func (r *registry) Get(name string) (AutoscalingEngine, error) {
	e, found := r.items[name]
	if !found {
		return nil, errors.Errorf("Autoscaling Engine '%s' not found", name)
	}

	return e, nil
}
