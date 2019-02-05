package digitalocean

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/containership/cerebral/pkg/autoscaling/engines/digitalocean/mocks"
	"github.com/digitalocean/godo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const nodePoolID = "node-pool-1-uuid"
const nodePoolName = "test-node-pool-1"

func newFakeNodePool(id, name string) *godo.KubernetesNodePool {
	return &godo.KubernetesNodePool{
		ID:    id,
		Name:  name,
		Count: 1,
		Nodes: []*godo.KubernetesNode{
			{
				ID:   "node-1",
				Name: "test-droplet",
			},
		},
	}
}

func newFakeOKResponse() *godo.Response {
	return &godo.Response{
		Response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewBufferString("test")),
		},
	}
}

// fakeAutoscalingEngine creates a fake autoscaling engine that can be used for
// testing digitalocean autoscaling engine functions
func fakeAutoscalingEngine() *Engine {
	client := godo.NewClient(nil)

	return &Engine{
		name: "containership",
		config: &cloudConfig{
			TokenEnvVarName: "TOKEN_ENV_VAR",
			ClusterID:       "cluster-uuid",
		},
		client: client,
	}
}

func TestNewClient(t *testing.T) {
	configuration := map[string]string{
		ConfigKeyTokenEnvVarName: "TOKEN_ENV_VAR",
		ConfigKeyClusterID:       "cluster-uuid",
	}
	os.Setenv(configuration[ConfigKeyTokenEnvVarName], "token")
	defer os.Unsetenv(configuration[ConfigKeyTokenEnvVarName])
	name := "digitalocean"

	_, err := NewClient("", configuration)
	assert.Error(t, err, "test error when no name is passed in")

	_, err = NewClient(name, configuration)
	assert.NoError(t, err, "testing new client passes")

	delete(configuration, ConfigKeyClusterID)
	_, err = NewClient(name, configuration)
	assert.Error(t, err, "test error when required config value is missing")
}

func TestName(t *testing.T) {
	c := fakeAutoscalingEngine()
	assert.Equal(t, c.name, c.Name())
}

func TestGetNodePoolByLabel(t *testing.T) {
	c := fakeAutoscalingEngine()

	_, err := c.getNodePoolByLabel(make(map[string]string, 0))
	assert.Error(t, err)
}

func TestSetTargetNodeCount(t *testing.T) {
	c := fakeAutoscalingEngine()

	emptyLabels := make(map[string]string, 0)

	result, err := c.SetTargetNodeCount(emptyLabels, -1, "")
	assert.Error(t, err, "Testing that an error is returned if there is a request to scale below 0")
	assert.False(t, result)

	result, err = c.SetTargetNodeCount(emptyLabels, 0, "strategy-dne")
	assert.Error(t, err, "testing that an error is returned if strategy doesn not exist")
	assert.False(t, result)

	nodepool := newFakeNodePool(nodePoolID, nodePoolName)
	nodepools := []*godo.KubernetesNodePool{nodepool}
	kmocks := mocks.KubernetesService{}
	c.client.Kubernetes = &kmocks

	kmocks.On("ListNodePools", mock.Anything, mock.Anything, mock.Anything).
		Return(nodepools, newFakeOKResponse(), nil).Twice()
	kmocks.On("UpdateNodePool", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nodepool, newFakeOKResponse(), nil).Times(7)

	result, err = c.SetTargetNodeCount(emptyLabels, 2, "")
	assert.NoError(t, err, "test that node pool is scaled")
	assert.True(t, result)

	result, err = c.SetTargetNodeCount(emptyLabels, 1, "")
	assert.NoError(t, err)
	assert.False(t, result, "test no scale action if cluster node number is desired node number")

	kmocks.On("ListNodePools", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, newFakeOKResponse(), errors.New("unable to list node pool")).Once()

	result, err = c.SetTargetNodeCount(emptyLabels, 1, "")
	assert.Error(t, err)
	assert.False(t, result, "test no scale action if error getting node pools")

	const key = "digitalocean.com/node-pool-id"
	label := map[string]string{
		key: nodePoolID,
	}
	c.config.NodePoolLabelKey = key

	kmocks.On("GetNodePool", mock.Anything, mock.Anything, mock.Anything).
		Return(nodepool, newFakeOKResponse(), nil).Twice()

	result, err = c.SetTargetNodeCount(label, 1, "")
	assert.NoError(t, err, "test no scale action on labeled node pool if desired node number is current node number")
	assert.False(t, result)

	result, err = c.SetTargetNodeCount(label, 2, "")
	assert.NoError(t, err)
	assert.True(t, result)

	kmocks.On("GetNodePool", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil, errors.New("node pool not found")).Once()

	label[key] = "bad-id"
	result, err = c.SetTargetNodeCount(label, 3, "")
	assert.Error(t, err)
	assert.False(t, result)

	// This check should be done last, the node pool that is added will persist in
	// subsequent tests
	nodepool2 := newFakeNodePool("node-pool-second-id", "second-node-pool-name")
	nodepool2.Count = 3
	nodepool3 := newFakeNodePool("node-pool-third-id", "third-node-pool-name")
	nodepool3.Count = 4
	nodepools = append(nodepools, nodepool2, nodepool3)

	kmocks.On("ListNodePools", mock.Anything, mock.Anything, mock.Anything).
		Return(nodepools, newFakeOKResponse(), nil)

	result, err = c.SetTargetNodeCount(emptyLabels, 7, "")
	assert.NoError(t, err, "testing scaling down a single node pool")
	assert.True(t, result)

	result, err = c.SetTargetNodeCount(emptyLabels, 3, "")
	assert.NoError(t, err, "testing scaling down multiple node pools")
	assert.True(t, result)

	result, err = c.SetTargetNodeCount(emptyLabels, 2, "")
	assert.Error(t, err, "testing that node pools scale down does not scale a node pool below 0")
	assert.False(t, result)

	kmocks.On("UpdateNodePool", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, newFakeOKResponse(), errors.New("update failed")).Twice()
	result, err = c.SetTargetNodeCount(emptyLabels, 3, "")
	assert.Error(t, err, "test that if node pool update fails error is returned")
	assert.False(t, result)

	kmocks.On("GetNodePool", mock.Anything, mock.Anything, mock.Anything).
		Return(nodepool, newFakeOKResponse(), nil).Once()
	result, err = c.SetTargetNodeCount(label, 2, "")
	assert.Error(t, err)
	assert.False(t, result)
}

func TestGetNodepoolCount(t *testing.T) {
	tests := []struct {
		curr   int
		total  int
		result int
	}{
		{5, 3, 2},
		{4, 8, 1},
	}

	for _, test := range tests {
		r := getMinNodesNeededInNodePoolCount(test.curr, test.total)
		assert.Equal(t, test.result, r)
	}
}

func TestGetScaleUpCount(t *testing.T) {
	tests := []struct {
		desired       int
		total         int
		nodePoolTotal int
		result        int
	}{
		{5, 3, 3, 5},
		{4, 3, 1, 2},
		{5, 3, 1, 3},
		{10, 4, 2, 8},
	}

	for _, test := range tests {
		r := getScaleUpCount(test.desired, test.total, test.nodePoolTotal)
		assert.Equal(t, test.result, r)
	}
}
