package containership

import (
	"math/rand"
	"os"
	"time"

	"github.com/pkg/errors"

	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/containership/cluster-manager/pkg/log"
	cscloud "github.com/containership/csctl/cloud"
	"github.com/containership/csctl/cloud/provision/types"

	"github.com/containership/cerebral/pkg/autoscaling"
)

const (
	nodePoolIDLabelKey = "containership.io/node-pool-id"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Engine returns an instance of the containership autoscaling engine
type Engine struct {
	name       string
	nodeLister corelistersv1.NodeLister
	cloud      cscloud.Interface
	config     *cloudConfig
}

// NewClient creates a new instance of the containership AutoScaling Engine, or an error
// It is expected that we should not modify the name or configuration here as the caller
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

	// TODO: is there anyway to test this without a real token?
	cloudclientset, err := cscloud.New(cscloud.Config{
		Token:            os.Getenv(config.TokenEnvVarName),
		ProvisionBaseURL: config.Address,
	})
	if err != nil {
		return nil, errors.New("unable to create containership cloud clientset")
	}

	return Engine{
		name:       name,
		config:     &config,
		cloud:      cloudclientset,
		nodeLister: nodeLister,
	}, nil
}

// Name returns the name of the engine
func (e Engine) Name() string {
	return e.name
}

// SetTargetNodeCount takes action to scale a target node pool
func (e Engine) SetTargetNodeCount(nodeSelectors map[string]string, numNodes int, strategy string) (bool, error) {
	if numNodes < 0 {
		return false, errors.New("cannot scale below 0")
	}

	log.Infof("Containership AutoscalingEngine %s is requesting Containership Cloud to set target nodes %v to %d", e.Name(), nodeSelectors, numNodes)

	switch strategy {
	case "random", "":
		// random is the default for this engine
		id, err := getRandomNodePoolIDToScale(nodeSelectors, nodePoolIDLabelKey, numNodes, e.nodeLister)
		if err != nil {
			return false, errors.Wrap(err, "Containership engine getting node pool ID to scale")
		}

		if id == "" {
			return false, nil
		}

		return e.scaleStrategyRandom(id, numNodes)
	default:
		return false, errors.Errorf("unable to scale node pool using strategy %s", strategy)
	}
}

// ScaleStrategyRandom take in the number of desired nodes for a node pool.
// It then makes a request to Containership Cloud API to set the node pool to
// the desired count
func (e Engine) scaleStrategyRandom(nodePoolID string, numNodes int) (bool, error) {
	target := int32(numNodes)
	req := types.ScaleNodePoolRequest{
		Count: &target,
	}

	_, err := e.cloud.Provision().NodePools(e.config.OrganizationID, e.config.ClusterID).Scale(nodePoolID, &req)
	if err != nil {
		return false, errors.Wrap(err, "There was an error scaling autoscaling group")
	}

	return true, nil
}
