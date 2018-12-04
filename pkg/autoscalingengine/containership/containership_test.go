package containership

import (
	"testing"

	"github.com/stretchr/testify/assert"

	cerebralv1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
)

// FakeAutoscalingEngine creates a fake autoscaling engine that can be used for
// testing containership autoscaling engine functions
func FakeAutoscalingEngine() *Engine {
	return &Engine{
		name: "containership",
		config: &cloudConfig{
			Address:         "https://provision-test.containership.io",
			TokenEnvVarName: "TOKEN_ENV_VAR",
			OrganizationID:  "organization-uuid",
			ClusterID:       "cluster-uuid",
		},
	}
}

func TestNewAutoscalingEngine(t *testing.T) {
	_, err := NewAutoscalingEngine(cerebralv1alpha1.AutoscalingEngine{})
	assert.Error(t, err)
}

func TestName(t *testing.T) {
	c := FakeAutoscalingEngine()
	assert.Equal(t, c.name, c.Name())
}

func TestSetTargetNodeCount(t *testing.T) {
	c := FakeAutoscalingEngine()

	emptyLabels := make(map[string]string, 0)

	result, err := c.SetTargetNodeCount(emptyLabels, -1, "")
	assert.Error(t, err, "Testing that an error is returned if there is a request to scale below 0")
	assert.False(t, result)

	result, err = c.SetTargetNodeCount(emptyLabels, 0, "")
	assert.Error(t, err, "Testing that an error is returned if there is a request to scale below 0")
	assert.False(t, result)

	nodePoolLabel := map[string]string{
		nodePoolIDLabelKey: "node-pool-uuid",
	}
	result, err = c.SetTargetNodeCount(nodePoolLabel, 2, "")
	assert.Error(t, err, "Testing that an error is returned if no strategy is specified")
	assert.False(t, result)
}
