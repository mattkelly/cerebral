package digitalocean

import (
	"context"
	"math/rand"
	"os"
	"time"

	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/digitalocean/godo"

	"github.com/containership/cerebral/pkg/autoscaling"
	"github.com/containership/cluster-manager/pkg/log"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	nodePoolIDLabelKey = "doks.digitalocean.com/node-pool-id"
)

// Engine is an instance of the DigitalOcean autoscaling engine
type Engine struct {
	name       string
	nodeLister corelistersv1.NodeLister
	client     *godo.Client
	config     *cloudConfig
}

// NewClient creates a new instance of the DigitalOcean Autoscaling Engine, or an error
// It is expected that we do not modify the name or configuration here as the caller
// may not have passed a DeepCopy
func NewClient(name string, configuration map[string]string, nodeLister corelistersv1.NodeLister) (autoscaling.Engine, error) {
	if name == "" {
		return nil, errors.New("name must be provided")
	}

	if nodeLister == nil {
		return nil, errors.New("node lister must be provided")
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
		name:       name,
		nodeLister: nodeLister,
		config:     &config,
		client:     doClient,
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

		scaled, err := e.scaleLabelSpecifiedNodePool(nodeSelectors, numNodes)
		if err != nil {
			return false, errors.Wrap(err, "unable to scale DigitalOcean cluster")
		}

		return scaled, nil

	default:
		return false, errors.Errorf("unknown scale strategy %s", strategy)
	}
}

func (e Engine) scaleLabelSpecifiedNodePool(nodeSelectors map[string]string, numNodes int) (bool, error) {
	id, err := getRandomNodePoolIDToScale(nodeSelectors, nodePoolIDLabelKey, numNodes, e.nodeLister)
	if err != nil {
		return false, errors.Wrap(err, "DigitalOcean engine getting node pool ID to scale")
	}

	if id == "" {
		return false, nil
	}

	np, _, err := e.client.Kubernetes.GetNodePool(context.Background(), e.config.ClusterID, id)
	if err != nil {
		return false, errors.Wrap(err, "getting node pool from DigitalOcean")
	}

	err = e.scaleNodePoolToCount(np, numNodes)
	if err != nil {
		return false, errors.Wrapf(err, "scaling node pool with node selectors %s", nodeSelectors)
	}

	return true, nil
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
	log.Infof("Requesting DigitalOcean to scale node pool %s to %d", req.Name, req.Count)

	_, _, err := e.client.Kubernetes.UpdateNodePool(context.Background(), e.config.ClusterID, nodePool.ID, &req)
	if err != nil {
		return errors.Wrap(err, "error scaling DigitalOcean node pool")
	}

	return nil
}
