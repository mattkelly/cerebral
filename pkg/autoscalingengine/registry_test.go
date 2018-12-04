package autoscalingengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test Engine to register
type TestEngine struct {
	name string
}

func NewTestAutoscalingEngine() *TestEngine {
	return &TestEngine{
		name: "tester",
	}
}

func (te *TestEngine) SetTargetNodeCount(nodeSelectorList map[string]string, numNodes int, strategy string) (bool, error) {
	return true, nil
}

func (te *TestEngine) Name() string {
	return te.name
}

// End Test engine

// This is the only test that should use the Registry() interface
func TestRegisty(t *testing.T) {
	r := Registry()
	te := NewTestAutoscalingEngine()
	r.Put(te)

	found := r.IsRegistered(te.name)
	assert.True(t, found)
}

func TestRegister(t *testing.T) {
	r := registry{
		items: make(map[string]AutoscalingEngine),
	}
	te := NewTestAutoscalingEngine()
	r.Put(te)

	found := r.IsRegistered(te.name)
	assert.True(t, found, "test engines are properly registered")

	engine, err := r.Get(te.name)
	assert.Nil(t, err)
	assert.Equal(t, te, engine)
}

func TestGet(t *testing.T) {
	r := registry{
		items: make(map[string]AutoscalingEngine),
	}
	te := NewTestAutoscalingEngine()
	r.Put(te)

	engine, err := r.Get(te.name)
	assert.Equal(t, te, engine, "test get engine returns the correct engine")
	assert.Nil(t, err)

	_, err = r.Get("invalidengine")
	assert.NotNil(t, err)
}

func TestIsRegistered(t *testing.T) {
	r := registry{
		items: make(map[string]AutoscalingEngine),
	}
	te := NewTestAutoscalingEngine()
	r.Put(te)

	e := r.IsRegistered(te.name)
	assert.True(t, e)

	e = r.IsRegistered("invalidengine")
	assert.False(t, e)
}
