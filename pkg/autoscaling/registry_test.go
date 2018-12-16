package autoscaling

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type stubEngine struct {
	name string
}

func (e stubEngine) Name() string {
	return e.name
}

func (e stubEngine) SetTargetNodeCount(nodeSelectorList map[string]string, numNodes int, strategy string) (bool, error) {
	return true, nil
}

var stub1 = stubEngine{name: "stub1"}
var stub2 = stubEngine{name: "stub2"}

func TestEngine(t *testing.T) {
	r := Registry()
	assert.NotNil(t, r)
}

func TestGet(t *testing.T) {
	// Don't use the actual registry, but instead instantiate a fresh underlying
	// registry type every test in order to bypass Put()
	r := registry{}

	_, err := r.Get("some-name")
	assert.Error(t, err, "error accessing empty registry")

	r = registry{
		items: map[string]Engine{
			"containership": stub1,
			"custom":        stub2,
		},
	}

	engine, err := r.Get("containership")
	assert.NoError(t, err, "get engine that exists")
	assert.Exactly(t, stub1, engine, "get returns correct backend")
	assert.NotEqual(t, stub2, engine, "get returns correct backend")

	engine, err = r.Get("custom")
	assert.NoError(t, err, "get engine that exists")
	assert.Exactly(t, stub2, engine, "get returns correct backend")
	assert.NotEqual(t, stub1, engine, "get returns correct backend")
}

func TestDelete(t *testing.T) {
	// Don't use the actual registry, but instead instantiate a fresh underlying
	// registry type every test in order to bypass Put()
	r := registry{
		items: map[string]Engine{
			"containership": stub1,
			"custom":        stub2,
		},
	}
	r.Delete("containership")
	assert.NotContains(t, "containership", "engine deleted properly")

	_, err := r.Get("containership")
	assert.Error(t, err, "get engine that was deleted")

	engine, err := r.Get("custom")
	assert.NoError(t, err, "non-deleted engine still exists")
	assert.Exactly(t, stub2, engine, "get returns correct engine")

	r.Delete("custom")
	assert.NotContains(t, "custom", "final engine deleted properly")
	assert.Empty(t, r.items, "registry emptied out cleanly")
}

func TestPut(t *testing.T) {
	r := registry{}

	// Trying to bring your own registry that's not initialized properly is no bueno
	assert.Panics(t, func() { r.Put("containership", stub1) }, "uninitialized registry panics")

	assert.NotPanics(t, func() { Registry().Put("containership", stub1) }, "real registry never panics")
	Registry().Delete("containership")
	assert.Empty(t, reg.items, "real registry emptied out cleanly")

	r = registry{
		items: make(map[string]Engine),
	}

	r.Put("containership", stub1)
	assert.Len(t, r.items, 1, "first element inserted")
	assert.Contains(t, r.items, "containership", "first element exists")

	engine, err := r.Get("containership")
	assert.NoError(t, err, "get engine that exists after put")
	assert.Exactly(t, stub1, engine, "get returns correct engine after put")

	r.Put("custom", stub2)
	assert.Len(t, r.items, 2, "another element inserted")
	assert.Contains(t, r.items, "custom", "another element exists")
}
