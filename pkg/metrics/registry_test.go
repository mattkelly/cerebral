package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
)

type stubBackend struct {
	name string
}

func (b stubBackend) GetValue(_ string, _ map[string]string, _ []*corev1.Node) (float64, error) {
	return 0, nil
}

var stub1 = stubBackend{name: "stub1"}
var stub2 = stubBackend{name: "stub2"}

func TestRegistry(t *testing.T) {
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
		items: map[string]Backend{
			"prometheus": stub1,
			"custom":     stub2,
		},
	}
	backend, err := r.Get("prometheus")
	assert.NoError(t, err, "get backend that exists")
	assert.Exactly(t, stub1, backend, "get returns correct backend")
	assert.NotEqual(t, stub2, backend, "get returns correct backend")

	backend, err = r.Get("custom")
	assert.NoError(t, err, "get backend that exists")
	assert.Exactly(t, stub2, backend, "get returns correct backend")
	assert.NotEqual(t, stub1, backend, "get returns correct backend")
}

func TestDelete(t *testing.T) {
	// Don't use the actual registry, but instead instantiate a fresh underlying
	// registry type every test in order to bypass Put()
	r := registry{
		items: map[string]Backend{
			"prometheus": stub1,
			"custom":     stub2,
		},
	}
	r.Delete("prometheus")
	assert.NotContains(t, "prometheus", "backend deleted properly")

	_, err := r.Get("prometheus")
	assert.Error(t, err, "get backend that was deleted")

	backend, err := r.Get("custom")
	assert.NoError(t, err, "non-deleted backend still exists")
	assert.Exactly(t, stub2, backend, "get returns correct backend")

	r.Delete("custom")
	assert.NotContains(t, "custom", "final backend deleted properly")
	assert.Empty(t, r.items, "registry emptied out cleanly")
}

func TestPut(t *testing.T) {
	r := registry{}

	// Trying to bring your own registry that's not initialized properly is no bueno
	assert.Panics(t, func() { r.Put("prometheus", stub1) }, "uninitialized registry panics")

	assert.NotPanics(t, func() { Registry().Put("prometheus", stub1) }, "real registry never panics")
	Registry().Delete("prometheus")
	assert.Empty(t, reg.items, "real registry emptied out cleanly")

	r = registry{
		items: make(map[string]Backend),
	}

	r.Put("prometheus", stub1)
	assert.Len(t, r.items, 1, "first element inserted")
	assert.Contains(t, r.items, "prometheus", "first element exists")

	backend, err := r.Get("prometheus")
	assert.NoError(t, err, "get backend that exists after put")
	assert.Exactly(t, stub1, backend, "get returns correct backend after put")

	r.Put("custom", stub2)
	assert.Len(t, r.items, 2, "another element inserted")
	assert.Contains(t, r.items, "custom", "another element exists")
}
