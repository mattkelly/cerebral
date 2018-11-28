package autoscalingengine

import (
	"testing"

	"k8s.io/apimachinery/pkg/labels"

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

func (te *TestEngine) SetTargetNodeCount(nodeSelectorList labels.Selector, numNodes int, strategy string) (bool, error) {
	return true, nil
}

func (te *TestEngine) Name() string {
	return te.name
}

// End Test engine

func TestRegister(t *testing.T) {
	ae := New()
	te := NewTestAutoscalingEngine()
	te2 := NewTestAutoscalingEngine()
	ae.Register(te)

	engine, found := ae.autoscalingEngines[te.name]

	assert.Equal(t, true, found, "test engines are properly registered")
	assert.Equal(t, te, engine)

	// testing registering engine with same name
	ae.Register(te2)
	engine, _ = ae.autoscalingEngines[te2.name]
	// engine should not have been overwritten
	assert.Equal(t, te, engine)
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

func TestIsRegistered(t *testing.T) {
	ae := New()
	te := NewTestAutoscalingEngine()
	ae.Register(te)

	e := ae.IsRegistered(te.name)
	assert.True(t, e)

	e = ae.IsRegistered("invalidengine")
	assert.False(t, e)
}
