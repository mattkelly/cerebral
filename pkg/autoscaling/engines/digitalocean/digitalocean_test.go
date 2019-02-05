package digitalocean

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/digitalocean/godo"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/containership/cerebral/pkg/autoscaling/engines/digitalocean/mocks"
	"github.com/containership/cerebral/pkg/kubernetestest"
)

const nodePoolID = "node-pool-1-uuid"
const nodePoolName = "test-node-pool-1"

func newFakeNodePool(id, name string, count int) *godo.KubernetesNodePool {
	return &godo.KubernetesNodePool{
		ID:    id,
		Name:  name,
		Count: count,
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

var (
	node0 = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "do-0",
			Labels: map[string]string{
				"digitalocean.com/node-pool-id": "node-pool-1-uuid",
			},
		},
	}
	node1 = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "do-1",
			Labels: map[string]string{
				"digitalocean.com/node-pool-id": "node-pool-1-uuid",
			},
		},
	}
)

// fakeAutoscalingEngine creates a fake autoscaling engine that can be used for
// testing digitalocean autoscaling engine functions
func fakeAutoscalingEngine(nodeLister corelistersv1.NodeLister) (*Engine, *mocks.KubernetesService) {
	kmocks := mocks.KubernetesService{}
	client := godo.NewClient(nil)
	client.Kubernetes = &kmocks

	return &Engine{
		name:       "digitalocean",
		nodeLister: nodeLister,
		config: &cloudConfig{
			TokenEnvVarName: "TOKEN_ENV_VAR",
			ClusterID:       "cluster-uuid",
		},
		client: client,
	}, &kmocks
}

func TestNewClient(t *testing.T) {
	nodeLister := kubernetestest.BuildNodeLister([]corev1.Node{node0, node1})
	configuration := map[string]string{
		ConfigKeyTokenEnvVarName: "TOKEN_ENV_VAR",
		ConfigKeyClusterID:       "cluster-uuid",
	}
	os.Setenv(configuration[ConfigKeyTokenEnvVarName], "token")
	defer os.Unsetenv(configuration[ConfigKeyTokenEnvVarName])
	name := "digitalocean"

	_, err := NewClient("", configuration, nil)
	assert.Error(t, err, "test error when no name is passed in")

	_, err = NewClient("name", configuration, nil)
	assert.Error(t, err, "test error when node lister not passed in")

	_, err = NewClient(name, configuration, nodeLister)
	assert.NoError(t, err, "testing new client passes")

	delete(configuration, ConfigKeyClusterID)
	_, err = NewClient(name, configuration, nodeLister)
	assert.Error(t, err, "test error when required config value is missing")
}

func TestName(t *testing.T) {
	c, _ := fakeAutoscalingEngine(nil)
	assert.Equal(t, c.name, c.Name())
}

func TestSetTargetNodeCountParamErrorCases(t *testing.T) {
	// set up fake engine
	nodeLister := kubernetestest.BuildNodeLister([]corev1.Node{})
	c, _ := fakeAutoscalingEngine(nodeLister)
	emptyLabels := map[string]string{}

	result, err := c.SetTargetNodeCount(emptyLabels, -1, "")
	assert.Error(t, err, "Testing that an error is returned if there is a request to scale below 0")
	assert.False(t, result)

	result, err = c.SetTargetNodeCount(emptyLabels, 0, "strategy-dne")
	assert.Error(t, err, "testing that an error is returned if strategy doesn not exist")
	assert.False(t, result)
}

func TestClusterAsNodePoolSetTargetNodeCount(t *testing.T) {
	nodeLister := kubernetestest.BuildNodeLister([]corev1.Node{node0, node1})
	c, kmocks := fakeAutoscalingEngine(nodeLister)

	// create DigitalOcean node pools for API responses
	nodepool := newFakeNodePool(nodePoolID, nodePoolName, 1)
	nodepools := []*godo.KubernetesNodePool{nodepool}

	emptyLabels := map[string]string{}

	kmocks.On("ListNodePools", mock.Anything, mock.Anything, mock.Anything).
		Return(nodepools, newFakeOKResponse(), nil).Twice()
	kmocks.On("UpdateNodePool", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nodepool, newFakeOKResponse(), nil)

	result, err := c.SetTargetNodeCount(emptyLabels, 2, "")
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
}

func TestSetTargetNodeCount(t *testing.T) {
	nodeLister := kubernetestest.BuildNodeLister([]corev1.Node{node0})
	c, kmocks := fakeAutoscalingEngine(nodeLister)

	nodepool := newFakeNodePool(nodePoolID, nodePoolName, 1)
	nodepools := []*godo.KubernetesNodePool{nodepool}

	const key = "digitalocean.com/node-pool-id"
	label := map[string]string{
		key: nodePoolID,
	}
	c.config.NodePoolLabelKey = key

	kmocks.On("GetNodePool", mock.Anything, mock.Anything, mock.Anything).
		Return(nodepool, newFakeOKResponse(), nil).Twice()
	kmocks.On("ListNodePools", mock.Anything, mock.Anything, mock.Anything).
		Return(nodepools, newFakeOKResponse(), nil)
	kmocks.On("UpdateNodePool", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nodepool, newFakeOKResponse(), nil)

	result, err := c.SetTargetNodeCount(label, 1, "")
	assert.Error(t, err, "test no scale action on labeled node pool if desired node number is current node number")
	assert.False(t, result)

	result, err = c.SetTargetNodeCount(label, 2, "")
	assert.NoError(t, err)
	assert.True(t, result)

	kmocks.On("GetNodePool", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil, errors.New("node pool not found")).Once()

	label[key] = "bad-id"
	result, err = c.SetTargetNodeCount(label, 5, "")
	assert.NoError(t, err)
	assert.False(t, result)
}

func TestMultipleNodePoolsSetTargetNodeCount(t *testing.T) {
	nodeLister := kubernetestest.BuildNodeLister([]corev1.Node{})
	c, kmocks := fakeAutoscalingEngine(nodeLister)

	nodepool := newFakeNodePool(nodePoolID, nodePoolName, 1)
	nodepools := []*godo.KubernetesNodePool{nodepool}

	emptyLabels := make(map[string]string, 0)

	// This check should be done last, the node pool that is added will persist in
	// subsequent tests
	nodepool2 := newFakeNodePool("node-pool-second-id", "second-node-pool-name", 3)
	nodepool3 := newFakeNodePool("node-pool-third-id", "third-node-pool-name", 4)
	nodepools = append(nodepools, nodepool2, nodepool3)

	kmocks.On("ListNodePools", mock.Anything, mock.Anything, mock.Anything).
		Return(nodepools, newFakeOKResponse(), nil)

	kmocks.On("UpdateNodePool", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nodepool, newFakeOKResponse(), nil).Times(5)

	result, err := c.SetTargetNodeCount(emptyLabels, 7, "")
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
