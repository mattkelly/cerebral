package containership

import (
	"testing"

	"github.com/stretchr/testify/assert"

	cerebralv1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
)

// fakeAutoscalingEngine creates a fake autoscaling engine that can be used for
// testing containership autoscaling engine functions
func fakeAutoscalingEngine() *Engine {
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
	c := fakeAutoscalingEngine()
	assert.Equal(t, c.name, c.Name())
}

func TestSetTargetNodeCount(t *testing.T) {
	c := fakeAutoscalingEngine()

	emptyLabels := make(map[string]string, 0)

	result, err := c.SetTargetNodeCount(emptyLabels, -1, "")
	assert.Error(t, err, "Testing that an error is returned if there is a request to scale below 0")
	assert.False(t, result)

	result, err = c.SetTargetNodeCount(emptyLabels, 0, "")
	assert.Error(t, err, "Testing that an error is returned if there is a request to scale below 0")
	assert.False(t, result)

	// TODO add a test checking that providing an empty string for strategy is ok
	// (containership engine should default) when Containership Cloud client is easily
	// mockable
}
