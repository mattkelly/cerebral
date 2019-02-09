package aws

import (
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	awsautoscaling "github.com/aws/aws-sdk-go/service/autoscaling"
	awsautoscalingiface "github.com/aws/aws-sdk-go/service/autoscaling/autoscalingiface"

	corev1 "k8s.io/api/core/v1"
	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/containership/cerebral/pkg/autoscaling"
	"github.com/containership/cerebral/pkg/nodeutil"
	"github.com/containership/cluster-manager/pkg/log"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Engine represents the AWS autoscaling engine; it implements autoscaling.Engine
type Engine struct {
	name string

	client awsautoscalingiface.AutoScalingAPI

	nodeLister corelistersv1.NodeLister
}

// NewClient creates a new instance of the containership AutoScaling Engine, or an error
// It is expected that we should not modify the name or configuration here as the caller
// may not have passed a DeepCopy
func NewClient(name string, nodeLister corelistersv1.NodeLister) (autoscaling.Engine, error) {
	if name == "" {
		return nil, errors.New("name must be provided")
	}

	if nodeLister == nil {
		return nil, errors.New("node lister must be provided")
	}

	// Note that aws-sdk-go pulls AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
	// directly from the environment
	sess, err := session.NewSession(aws.NewConfig().WithRegion(getRegion()))
	if err != nil {
		return nil, errors.Wrap(err, "creating new AWS session")
	}

	client := awsautoscaling.New(sess)

	return &Engine{
		name:       name,
		client:     client,
		nodeLister: nodeLister,
	}, nil
}

// Name returns the name of the engine
func (e Engine) Name() string {
	return e.name
}

// SetTargetNodeCount takes action to scale a target node pool
func (e Engine) SetTargetNodeCount(nodeSelector map[string]string, numNodes int, strategy string) (bool, error) {
	if numNodes < 0 {
		return false, errors.New("cannot scale below 0")
	}

	selector := nodeutil.GetNodesLabelSelector(nodeSelector)
	nodes, err := e.nodeLister.List(selector)
	if err != nil {
		return false, errors.Wrap(err, "listing nodes")
	}

	if len(nodes) == 0 {
		log.Infof("zero nodes selected by selector %s", nodeSelector)
		return false, nil
	}

	switch strategy {
	case "random", "":
		// random is the default for this engine
		return e.scaleStrategyRandom(nodes, numNodes)

	default:
		return false, errors.Errorf("unknown scale strategy %s", strategy)
	}

}

func (e Engine) scaleStrategyRandom(nodes []*corev1.Node, numNodes int) (bool, error) {
	selectedNode := nodes[rand.Intn(len(nodes))]
	providerID := selectedNode.Spec.ProviderID
	if providerID == "" {
		return false, errors.Errorf("selected node %s does not have providerID available", selectedNode.Name)
	}

	instanceID := instanceIDFromProviderID(providerID)

	log.Debugf("Selected node %s has providerID %q, instanceID %q", selectedNode.Name, providerID, instanceID)

	asgName, err := e.getAutoscalingGroupNameForInstanceID(instanceID)
	if err != nil {
		return false, err
	}

	log.Infof("AWS AutoscalingEngine %s is requesting AWS to scale to %d", e.Name(), numNodes)

	_, err = e.client.SetDesiredCapacity(&awsautoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: asgName,
		DesiredCapacity:      aws.Int64(int64(numNodes)),
		HonorCooldown:        aws.Bool(false),
	})
	if err != nil {
		return false, errors.Wrapf(err, "setting desired capacity of %q to %d in AWS", *asgName, numNodes)
	}

	return true, nil
}

func (e Engine) getAutoscalingGroupNameForInstanceID(instanceID string) (*string, error) {
	result, err := e.client.DescribeAutoScalingInstances(&awsautoscaling.DescribeAutoScalingInstancesInput{
		InstanceIds: []*string{
			aws.String(instanceID),
		},
	})

	if err != nil {
		return nil, errors.Wrap(err, "describing autoscaling instances in AWS")
	}

	if result == nil {
		return nil, errors.New("AWS returned nil result for describe autoscaling instances")
	}

	if len(result.AutoScalingInstances) != 1 {
		// We passed exactly one instance to list, so we better get exactly one back
		return nil, errors.Errorf("expected exactly 1 autoscaling instance but AWS returned %d", len(result.AutoScalingInstances))
	}

	return result.AutoScalingInstances[0].AutoScalingGroupName, nil
}

// Get the AWS instance ID from a provider ID.
// This does not perform any validation; we'll just rely on AWS to validate it
func instanceIDFromProviderID(providerID string) string {
	fields := strings.Split(providerID, "/")
	return fields[len(fields)-1]
}

// Get the current AWS region, either from the environment or from the EC2
// metadata service if the env var is not set.
// This function is borrowed from https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler
func getRegion(cfg ...*aws.Config) string {
	region, present := os.LookupEnv("AWS_REGION")
	if !present {
		svc := ec2metadata.New(session.New(), cfg...)
		if r, err := svc.Region(); err == nil {
			region = r
		}
	}
	return region
}
