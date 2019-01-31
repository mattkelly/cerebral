package containership

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/stretchr/testify/assert"

	"github.com/containership/cerebral/pkg/kubernetestest"
)

var (
	node0 = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prom-0",
		},
	}
	node1 = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prom-1",
		},
	}
)

// fakeAutoscalingEngine creates a fake autoscaling engine that can be used for
// testing containership autoscaling engine functions
func fakeAutoscalingEngine(nodeLister corelistersv1.NodeLister) *Engine {
	return &Engine{
		name:       "containership",
		nodeLister: nodeLister,
		config: &cloudConfig{
			Address:         "https://provision-test.containership.io",
			TokenEnvVarName: "TOKEN_ENV_VAR",
			OrganizationID:  "organization-uuid",
			ClusterID:       "cluster-uuid",
		},
	}
}

func TestNewClient(t *testing.T) {
	name := "containership"
	configuration := map[string]string{
		"address":         "https://provision-test.containership.io",
		"tokenEnvVarName": "TOKEN_ENV_VAR",
		"organizationID":  "organization-uuid",
		"clusterID":       "cluster-uuid",
	}
	nodeLister := kubernetestest.BuildNodeLister([]corev1.Node{node0, node1})

	copiedConfiguration := map[string]string{}

	for key, value := range configuration {
		copiedConfiguration[key] = value
	}

	_, err := NewClient(name, copiedConfiguration, nodeLister)
	assert.True(t, reflect.DeepEqual(copiedConfiguration, configuration), "Testing that arguments are not modified")

	_, err = NewClient(name, configuration, nodeLister)
	assert.Error(t, err, "Testing that an error is returned when the token environment variable is not defined")

	os.Setenv(configuration["tokenEnvVarName"], "token")
	c, err := NewClient(name, configuration, nodeLister)
	assert.NoError(t, err, "Testing that no error is returned when client is successfully created")
	assert.NotNil(t, c, "Testing that client is not nil when successfully created")
	os.Unsetenv(configuration["tokenEnvVarName"])

	for key := range configuration {
		existingValue := configuration[key]
		delete(configuration, key)
		_, err = NewClient(name, configuration, nodeLister)
		assert.Error(t, err, fmt.Sprintf("Testing that an error is returned when client configuration is missing %q", key))
		configuration[key] = existingValue
	}
}

func TestName(t *testing.T) {
	c := fakeAutoscalingEngine(nil)
	assert.Equal(t, c.name, c.Name())
}

func TestSetTargetNodeCount(t *testing.T) {
	nodeLister := kubernetestest.BuildNodeLister([]corev1.Node{node0, node1})
	c := fakeAutoscalingEngine(nodeLister)

	emptyLabels := make(map[string]string, 0)

	result, err := c.SetTargetNodeCount(emptyLabels, -1, "")
	assert.Error(t, err, "testing that an error is returned if there is a request to scale below 0")
	assert.False(t, result)

	result, err = c.SetTargetNodeCount(emptyLabels, 0, "")
	assert.Error(t, err, "testing that an error is returned if there is a request to scale below 0")
	assert.False(t, result)

	selector := map[string]string{
		"nonode": "selector",
	}
	result, err = c.SetTargetNodeCount(selector, 2, "")
	assert.NoError(t, err, "testing that no error or scale event when no nodes are selected")
	assert.False(t, result)

	// TODO add a test checking that providing an empty string for strategy is ok
	// (containership engine should default) when Containership Cloud client is easily
	// mockable
}

func TestDefaultAndValidate(t *testing.T) {
	c := cloudConfig{}
	err := c.defaultAndValidate(nil)
	assert.Error(t, err, "nil config provided is invalid")

	c = cloudConfig{}
	err = c.defaultAndValidate(map[string]string{})
	assert.Error(t, err, "empty config provided is invalid")

	configuration := map[string]string{
		"address":         "https://provision-test.containership.io",
		"tokenEnvVarName": "TOKEN_ENV_VAR",
		"organizationID":  "organization-uuid",
		"clusterID":       "cluster-uuid",
	}

	os.Setenv(configuration["tokenEnvVarName"], "token")

	c = cloudConfig{}
	err = c.defaultAndValidate(configuration)
	assert.NoError(t, err, "good config")
	assert.Equal(t, "https://provision-test.containership.io", c.Address, "address not defaulted if provided")

	os.Unsetenv(configuration["tokenEnvVarName"])

	configurationWithoutAddress := map[string]string{
		"tokenEnvVarName": "TOKEN_ENV_VAR",
		"organizationID":  "organization-uuid",
		"clusterID":       "cluster-uuid",
	}

	os.Setenv(configurationWithoutAddress["tokenEnvVarName"], "token")

	c = cloudConfig{}
	err = c.defaultAndValidate(configurationWithoutAddress)
	assert.NoError(t, err, "good config")
	assert.Equal(t, "https://provision.containership.io", c.Address, "address defaulted if not provided")

	for key := range configurationWithoutAddress {
		c = cloudConfig{}
		existingValue := configurationWithoutAddress[key]
		delete(configurationWithoutAddress, key)
		err = c.defaultAndValidate(configurationWithoutAddress)
		assert.Error(t, err, fmt.Sprintf("Testing that an error is returned when client configuration is missing %q", key))
		configurationWithoutAddress[key] = existingValue
	}
	os.Unsetenv(configurationWithoutAddress["tokenEnvVarName"])
}
