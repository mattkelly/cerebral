package autoscaling

import (
	"sync"

	"github.com/pkg/errors"
)

// RegistryInterface is an interface to an Engine registry.
// See function comments for implementation limitations.
type RegistryInterface interface {
	Get(name string) (Engine, error)
	Delete(name string)
	Put(name string, engine Engine)
}

type registry struct {
	sync.RWMutex
	items map[string]Engine
}

var reg = registry{
	items: make(map[string]Engine),
}

// Registry provides an interface to the single Engine registry.
func Registry() RegistryInterface {
	return &reg
}

// Get returns the Engine with the given name from the registry, or an error
// if it does not exist. An Engine returned is not guaranteed to be valid
// still; it's assumed that the caller will handle Engine errors and delete it
// from the Registry if appropriate.
func (r *registry) Get(name string) (Engine, error) {
	r.RLock()
	defer r.RUnlock()

	var engine Engine
	var ok bool
	if engine, ok = r.items[name]; !ok {
		return nil, errors.Errorf("engine %q does not exist", name)
	}

	return engine, nil
}

// Delete deletes the Engine with the given name from the registry, or noops
// if the Engine doesn't exist. It only deletes it from the registry; it does
// not clean up the underlying type.
func (r *registry) Delete(name string) {
	r.Lock()
	defer r.Unlock()

	delete(r.items, name)
}

// Put puts an Engine with the given name into the registry. If an Engine
// already exists with the given name, it will simply be overwritten.
func (r *registry) Put(name string, engine Engine) {
	r.Lock()
	defer r.Unlock()

	r.items[name] = engine
}
