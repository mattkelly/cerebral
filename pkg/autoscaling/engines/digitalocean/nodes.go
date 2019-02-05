package digitalocean

import (
	"math/rand"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"

	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/containership/cerebral/pkg/nodeutil"
)

// returns all nodes that match the passed in node selector
func getASGNodes(selector map[string]string, nodeLister corelistersv1.NodeLister) ([]*corev1.Node, error) {
	if nodeLister == nil {
		return nil, errors.New("node lister cannot be nil")
	}

	ns := nodeutil.GetNodesLabelSelector(selector)

	return nodeLister.List(ns)
}

// selects all the nodes that match the passed in node selector
// it then checks for any errors that could have happened selecting nodes
// and finally returns the ID for the node pool that should be scaled
func getRandomNodePoolIDToScale(
	nodeSelectors map[string]string,
	knownLabelKey string,
	numNodes int,
	nodeLister corelistersv1.NodeLister) (string, error) {
	// get all nodes that are selected by the passed in node selector
	nodes, err := getASGNodes(nodeSelectors, nodeLister)
	if err != nil {
		return "", errors.Wrap(err, "unable to list nodes")
	}

	numSelectedNodes := len(nodes)
	if numSelectedNodes == 0 {
		return "", nil
	}

	if numSelectedNodes == numNodes {
		return "", errors.New("can not scale to current count")
	}

	// get a random node in the node pool
	node := nodes[rand.Intn(numSelectedNodes)]
	// get the node pool ID from the previously selected node
	id, ok := node.Labels[knownLabelKey]
	if !ok {
		return "", errors.New("does not contain known label key")
	}

	return id, nil
}
