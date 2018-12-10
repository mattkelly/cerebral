package containership

import (
	"os"

	cscloud "github.com/containership/csctl/cloud"
	"github.com/containership/csctl/cloud/provision/types"

	"github.com/containership/cluster-manager/pkg/log"

	cerebralv1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"

	"github.com/pkg/errors"
)

const (
	nodePoolIDLabelKey = "containership.io/node-pool-id"
)

// Engine returns an instance of the containership autoscaling engine
type Engine struct {
	name   string
	cloud  cscloud.Interface
	config *cloudConfig
}

type cloudConfig struct {
	Address         string
	TokenEnvVarName string
	OrganizationID  string
	ClusterID       string
}

// NewAutoscalingEngine creates a new instance of the containership AutoScaling
// engine
func NewAutoscalingEngine(e cerebralv1alpha1.AutoscalingEngine) (*Engine, error) {
	configmap := e.Spec.Configuration
	engine := &Engine{
		name: e.Name,
		config: &cloudConfig{
			Address:         configmap["address"],
			TokenEnvVarName: configmap["tokenEnvVarName"],
			OrganizationID:  configmap["organizationID"],
			ClusterID:       configmap["clusterID"],
		},
	}

	token := os.Getenv(engine.config.TokenEnvVarName)
	if token == "" {
		return nil, errors.New("unable to get Containership Cloud API cluster token")
	}

	// TODO: is there anyway to test this without a real token?
	cloudclientset, err := cscloud.New(cscloud.Config{
		Token:            token,
		ProvisionBaseURL: engine.config.Address,
	})
	if err != nil {
		return nil, errors.New("unable to create containership cloud clientset")
	}

	engine.cloud = cloudclientset

	return engine, nil
}

// SetTargetNodeCount takes action to scale a target node pool
func (cae *Engine) SetTargetNodeCount(nodeSelectors map[string]string, numNodes int, strategy string) (bool, error) {
	if numNodes < 0 {
		return false, errors.New("cannot scale below 0")
	}

	id, found := nodeSelectors[nodePoolIDLabelKey]
	if !found {
		return false, errors.New("could not get autoscaling group node pool ID")
	}

	log.Infof("AutoscalingEngine %s is requesting Containership Cloud to set target nodes %v to %d", cae.Name(), nodeSelectors, numNodes)
	switch strategy {
	case "random", "":
		// random is the default for this engine
		return cae.scaleStrategyRandom(id, numNodes)
	default:
		return false, errors.Errorf("unable to scale node pool using strategy %s", strategy)
	}
}

// Name returns the name of the engine
func (cae *Engine) Name() string {
	return cae.name
}

// ScaleStrategyRandom take in the number of desired nodes for a node pool.
// It then makes a request to Containership Cloud API to set the node pool to
// the desired count
func (cae *Engine) scaleStrategyRandom(nodePoolID string, numNodes int) (bool, error) {
	target := int32(numNodes)
	req := types.ScaleNodePoolRequest{
		Count: &target,
	}

	_, err := cae.cloud.Provision().NodePools(cae.config.OrganizationID, cae.config.ClusterID).Scale(nodePoolID, &req)
	if err != nil {
		return false, errors.Wrap(err, "There was an error scaling autoscaling group")
	}

	return true, nil
}
