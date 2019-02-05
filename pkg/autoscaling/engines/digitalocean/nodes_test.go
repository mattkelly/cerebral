package digitalocean

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"

	"github.com/containership/cerebral/pkg/kubernetestest"
)

var (
	nodeselection0 = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-0",
			Labels: map[string]string{
				"region": "us-east",
			},
		},
	}
	nodeselection1 = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			Labels: map[string]string{
				"region": "us-west",
			},
		},
	}
)

func TestGetASGNodes(t *testing.T) {
	_, err := getASGNodes(map[string]string{}, nil)
	assert.Error(t, err)

	nl := kubernetestest.BuildNodeLister([]corev1.Node{nodeselection0, nodeselection1})
	nodes, err := getASGNodes(map[string]string{
		"region": "us-east",
	}, nl)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(nodes))
}

func TestGetNodePoolIDToScale(t *testing.T) {
	nl := kubernetestest.BuildNodeLister([]corev1.Node{nodeselection0, nodeselection1})
	regionSelector := map[string]string{
		"region": "us-east",
	}
	knownLabelKey := "region"

	id, err := getRandomNodePoolIDToScale(regionSelector, knownLabelKey, 2, nl)
	assert.NoError(t, err)
	assert.Equal(t, "us-east", id)

	_, err = getRandomNodePoolIDToScale(regionSelector, "unknownkey", 2, nl)
	assert.Error(t, err, "test node labels don't match key")

	emptySelector := map[string]string{}
	_, err = getRandomNodePoolIDToScale(emptySelector, knownLabelKey, 2, nl)
	assert.Error(t, err, "test current nodes equals desired nodes")

	selectorSelectsNothing := map[string]string{
		"region": "canada",
	}
	id, err = getRandomNodePoolIDToScale(selectorSelectsNothing, knownLabelKey, 2, nl)
	assert.NoError(t, err, "test no nodes selected  error")
	assert.Equal(t, "", id)

	_, err = getRandomNodePoolIDToScale(emptySelector, knownLabelKey, 2, nil)
	assert.Error(t, err, "test nil label selector")
}
