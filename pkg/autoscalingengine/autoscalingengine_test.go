package autoscalingengine

import (
	"testing"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/stretchr/testify/assert"
)

// TODO tests following functions
// RegisterAutoscalingEngine
// Exists
// Get

// Test Engine to register
type TestEngine struct {
	name string
}

func NewTestAutoscalingEngine() *TestEngine {
	return &TestEngine{
		name: "tester",
	}
}

func (te *TestEngine) SetTargetNodeCount(nodeSelectorList labels.Selector, numNodes int, heuristic string) (bool, error) {
	return true, nil
}

func (te *TestEngine) Name() string {
	return te.name
}

// End Test engine

func TestRegister(t *testing.T) {
	ae := New()
	te := NewTestAutoscalingEngine()
	ae.Register(te)

	_, found := ae.autoscalingEngines[te.name]

	assert.Equal(t, true, found, "test engines are properly registered")
}

func TestGet(t *testing.T) {
	ae := New()
	te := NewTestAutoscalingEngine()
	ae.Register(te)

	engine, err := ae.Get(te.name)
	assert.Equal(t, te, engine, "test get engine returns the correct engine")
	assert.Nil(t, err)

	_, err = ae.Get("invalidengine")
	assert.NotNil(t, err)
}

func TestExists(t *testing.T) {
	ae := New()
	te := NewTestAutoscalingEngine()
	ae.Register(te)

	e := ae.Exists(te.name)
	assert.True(t, e)

	e = ae.Exists("invalidengine")
	assert.False(t, e)
}
