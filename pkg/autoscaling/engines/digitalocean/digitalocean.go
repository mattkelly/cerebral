package digitalocean

import (
	"context"
	"math/rand"
	"os"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/digitalocean/godo"

	"github.com/containership/cerebral/pkg/autoscaling"
	"github.com/containership/cluster-manager/pkg/log"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Engine is an instance of the DigitalOcean autoscaling engine
type Engine struct {
	name   string
	client *godo.Client
	config *cloudConfig
}

// NewClient creates a new instance of the DigitalOcean Autoscaling Engine, or an error
// It is expected that we do not modify the name or configuration here as the caller
// may not have passed a DeepCopy
func NewClient(name string, configuration map[string]string) (autoscaling.Engine, error) {
	if name == "" {
		return nil, errors.New("name must be provided")
	}

	config := cloudConfig{}
	if err := config.defaultAndValidate(configuration); err != nil {
		return nil, errors.Wrap(err, "validating configuration")
	}

	token := os.Getenv(config.TokenEnvVarName)
	oauthClient := oauth2.NewClient(oauth2.NoContext, &tokenSource{
		AccessToken: token,
	})
	doClient, err := godo.New(oauthClient)
	if err != nil {
		return nil, errors.Wrap(err, "creating godo client")
	}

	e := Engine{
		name:   name,
		config: &config,
		client: doClient,
	}

	return e, nil
}

// Name returns the name of the engine
func (e Engine) Name() string {
	return e.name
}

// SetTargetNodeCount takes action to scale the target node pool
func (e Engine) SetTargetNodeCount(nodeSelectors map[string]string, numNodes int, strategy string) (bool, error) {
	if numNodes < 0 {
		return false, errors.New("cannot scale below 0")
	}

	log.Infof("DigitalOcean AutoscalingEngine %s is requesting DigitalOcean to scale to %d", e.Name(), numNodes)

	switch strategy {
	// random is the default for this engine
	case "random", "":
		var scaled bool
		var err error
		if len(nodeSelectors) > 0 {
			// if a node selector is provided we should only look at that node pool
			scaled, err = e.scaleLabelSpecifiedNodePool(nodeSelectors, numNodes)
		} else {
			// try scaling any node pool in the DigitalOcean cluster
			scaled, err = e.scaleRandomNodePool(numNodes)
		}

		if err != nil {
			return false, errors.Wrap(err, "unable to scale DigitalOcean cluster")
		}

		return scaled, nil

	default:
		return false, errors.Errorf("unknown scale strategy %s", strategy)
	}
}

func (e Engine) scaleLabelSpecifiedNodePool(nodeSelectors map[string]string, numNodes int) (bool, error) {
	np, err := e.getNodePoolByLabel(nodeSelectors)
	if err != nil {
		return false, err
	}

	if np.Count == numNodes {
		return false, nil
	}

	err = e.scaleNodePoolToCount(np, numNodes)
	if err != nil {
		return false, errors.Wrapf(err, "unable to scale node pool with node selectors %s", nodeSelectors)
	}

	return true, nil
}

// DigitalOcean node pools currently do not have the labels needed to identify which
// group a node belongs to, making scaling a particular group impossible.
// Scale Up: get a random node pool and scale it to the set count
// Scale Down: we need to split the number of nodes that needs to be scaled down
// across node pools if the scale down action will make a node pool less than 1 node
func (e Engine) scaleRandomNodePool(numNodes int) (bool, error) {
	nps, err := e.listNodePools()
	if err != nil {
		return false, errors.Wrap(err, "unable to list node pools")
	}

	var total int
	for _, np := range nps {
		total += np.Count
	}

	switch {
	case total < numNodes:
		i := rand.Intn(len(nps))
		np := nps[i]
		scaleUpBy := getScaleUpCount(numNodes, total, np.Count)
		err = e.scaleNodePoolToCount(np, scaleUpBy)

	case total > numNodes:
		scaleDownBy := total - numNodes
		err = e.randomScaleDown(nps, scaleDownBy)

	default:
		return false, nil
	}

	if err != nil {
		return false, errors.Wrap(err, "unable to scale random node pool")
	}

	return true, nil
}

// find the node pools total count for it to scale to the desired scale up count
func getScaleUpCount(desired, total, nodePoolTotal int) int {
	return (desired - total) + nodePoolTotal
}

// we shuffle the node pool array that is passed in order to not only scale
// down the first node pool for every scale down request
func (e Engine) randomScaleDown(nodepools []*godo.KubernetesNodePool, numToScale int) error {
	nodepools = shuffle(nodepools)
	for _, np := range nodepools {
		if numToScale == 0 {
			break
		}

		// limitations in DigitalOcean prevent scaling a node pool to less than 1
		if np.Count == 1 {
			continue
		}

		scaleNodePoolTo := getMinNodesNeededInNodePoolCount(np.Count, numToScale)
		err := e.scaleNodePoolToCount(np, scaleNodePoolTo)
		if err != nil {
			return err
		}

		numToScale = numToScale - (np.Count - scaleNodePoolTo)
	}

	// this case can happen if the total number of desired nodes is less than the
	// number of node pools in the cluster since there has to be 1 node per node pool
	if numToScale != 0 {
		return errors.New("unable to scale to desired node count")
	}

	return nil
}

func shuffle(nodepools []*godo.KubernetesNodePool) []*godo.KubernetesNodePool {
	ret := make([]*godo.KubernetesNodePool, len(nodepools))
	perm := rand.Perm(len(nodepools))
	for i, randIndex := range perm {
		ret[i] = nodepools[randIndex]
	}
	return ret
}

func getMinNodesNeededInNodePoolCount(curr, total int) int {
	if curr > total {
		return curr - total
	}

	return 1
}

func (e Engine) listNodePools() ([]*godo.KubernetesNodePool, error) {
	opts := godo.ListOptions{}
	nodepools, _, err := e.client.Kubernetes.ListNodePools(context.Background(), e.config.ClusterID, &opts)
	if err != nil {
		return nil, err
	}

	return nodepools, nil
}

// getNodePoolByLabel uses the key assigned to 'NodePoolLabelKey' in the configuration
// to get the DigitalOcean node pool by ID
func (e Engine) getNodePoolByLabel(nodeSelectors map[string]string) (*godo.KubernetesNodePool, error) {
	poolID, ok := nodeSelectors[e.config.NodePoolLabelKey]
	if !ok {
		return nil, errors.New("node pool selector does not contain node pool key")
	}

	nodepool, _, err := e.client.Kubernetes.GetNodePool(context.Background(), e.config.ClusterID, poolID)
	if err != nil {
		return nil, err
	}

	return nodepool, nil
}

// takes in the number of desired nodes for a node pool. This can either scale up
// or scale down the node pool and DigitalOcean will choose which node to delete
// in the scale down case
func (e Engine) scaleNodePoolToCount(nodePool *godo.KubernetesNodePool, numNodes int) error {
	// create a request to scale node pool
	// both name and count are required fields
	req := godo.KubernetesNodePoolUpdateRequest{
		Name:  nodePool.Name,
		Count: numNodes,
	}
	log.Infof("Requesting DigitalOcean to scale node pool %s to %s", req.Name, req.Count)

	_, _, err := e.client.Kubernetes.UpdateNodePool(context.Background(), e.config.ClusterID, nodePool.ID, &req)
	if err != nil {
		return errors.Wrap(err, "error scaling DigitalOcean node pool")
	}

	return nil
}
