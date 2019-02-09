package aws

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/aws/aws-sdk-go/aws"
	awsautoscaling "github.com/aws/aws-sdk-go/service/autoscaling"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/containership/cerebral/pkg/autoscaling/engines/aws/mocks"
	"github.com/containership/cerebral/pkg/kubernetestest"
)

const providerID0 = "aws:///us-east-1a/i-0a2ade0106d44fd46"
const providerID1 = "aws:///us-east-1a/i-01234567890123456"

var (
	node0 = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-0",
			Labels: map[string]string{
				"test": "",
			},
		},
		Spec: corev1.NodeSpec{
			ProviderID: providerID0,
		},
	}

	node1 = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-1",
			Labels: map[string]string{
				"test": "",
			},
		},
		Spec: corev1.NodeSpec{
			ProviderID: providerID1,
		},
	}

	nodeWithoutProviderID = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "no-provider-id",
			Labels: map[string]string{
				"no-provider-id": "",
			},
		},
	}
)

func TestNewClient(t *testing.T) {
	_, err := NewClient("", nil)
	assert.Error(t, err, "name is required")

	_, err = NewClient("test", nil)
	assert.Error(t, err, "NodeLister is required")

	nl := kubernetestest.BuildNodeLister([]corev1.Node{node0, node1})
	c, err := NewClient("test", nl)
	assert.NoError(t, err)
	assert.NotNil(t, c)
}

func TestName(t *testing.T) {
	nl := kubernetestest.BuildNodeLister([]corev1.Node{node0, node1})
	e, err := NewClient("aws", nl)
	assert.NoError(t, err)
	assert.Equal(t, "aws", e.Name())
}

func TestSetTargetNodeCount(t *testing.T) {
	os.Setenv("AWS_REGION", "us-east-1")
	defer os.Unsetenv("AWS_REGION")

	nl := kubernetestest.BuildNodeLister([]corev1.Node{node0, node1, nodeWithoutProviderID})
	mockAPI := mocks.AutoScalingAPI{}
	e := Engine{
		name:       "test",
		client:     &mockAPI,
		nodeLister: nl,
	}

	emptyLabels := make(map[string]string, 0)

	_, err := e.SetTargetNodeCount(emptyLabels, -1, "")
	assert.Error(t, err, "error is returned if there is a request to scale below 0")

	_, err = e.SetTargetNodeCount(emptyLabels, 1, "not a strategy")
	assert.Error(t, err, "error is returned for unknown strategy")

	result, err := e.SetTargetNodeCount(map[string]string{"select": "nothing"}, 2, "")
	assert.NoError(t, err, "no error if zero nodes selected")
	assert.False(t, result, "no action if zero nodes selected")

	_, err = e.SetTargetNodeCount(map[string]string{"no-provider-id": ""}, 3, "")
	assert.Error(t, err, "error if a selected node does not have provider ID")

	// Different failure cases for DescribeAutoScalingInstances are tested elsewhere,
	// so just return a good result here
	mockAPI.On("DescribeAutoScalingInstances", mock.Anything).
		Return(&awsautoscaling.DescribeAutoScalingInstancesOutput{
			AutoScalingInstances: []*awsautoscaling.InstanceDetails{
				{
					AutoScalingGroupName: aws.String("test"),
				},
			},
		}, nil)

	mockAPI.On("SetDesiredCapacity", mock.Anything).
		Return(nil, errors.New("some error")).
		Once()

	_, err = e.SetTargetNodeCount(map[string]string{"test": ""}, 3, "")
	assert.Error(t, err, "error if set desired capacity fails")

	mockAPI.On("SetDesiredCapacity", mock.Anything).
		Return(nil, nil)

	_, err = e.SetTargetNodeCount(map[string]string{"test": ""}, 3, "")
	assert.NoError(t, err, "successful scale request")
}

func TestGetAutoscalingGroupNameForInstanceID(t *testing.T) {
	os.Setenv("AWS_REGION", "us-east-1")
	defer os.Unsetenv("AWS_REGION")

	nl := kubernetestest.BuildNodeLister([]corev1.Node{node0, node1, nodeWithoutProviderID})
	mockAPI := mocks.AutoScalingAPI{}
	e := Engine{
		name:       "test",
		client:     &mockAPI,
		nodeLister: nl,
	}

	mockAPI.On("DescribeAutoScalingInstances", mock.Anything).
		Return(nil, errors.New("some error")).
		Once()

	const instanceID = "i-01234567890123456"
	_, err := e.getAutoscalingGroupNameForInstanceID(instanceID)
	assert.Error(t, err, "error if describe autoscaling instances fails")

	mockAPI.On("DescribeAutoScalingInstances", mock.Anything).
		Return(nil, nil).
		Once()

	_, err = e.getAutoscalingGroupNameForInstanceID(instanceID)
	assert.Error(t, err, "error if describe autoscaling instances returns nil result")

	mockAPI.On("DescribeAutoScalingInstances", mock.Anything).
		Return(&awsautoscaling.DescribeAutoScalingInstancesOutput{}, nil).
		Once()

	_, err = e.getAutoscalingGroupNameForInstanceID(instanceID)
	assert.Error(t, err, "error if describe autoscaling instances returns zero instances")

	mockAPI.On("DescribeAutoScalingInstances", mock.Anything).
		Return(&awsautoscaling.DescribeAutoScalingInstancesOutput{
			AutoScalingInstances: []*awsautoscaling.InstanceDetails{
				{
					AutoScalingGroupName: aws.String("one"),
				},
				{
					AutoScalingGroupName: aws.String("two"),
				},
			},
		}, nil).
		Once()

	_, err = e.getAutoscalingGroupNameForInstanceID(instanceID)
	assert.Error(t, err, "error if describe autoscaling instances returns more than 1 instance")

	mockAPI.On("DescribeAutoScalingInstances", mock.Anything).
		Return(&awsautoscaling.DescribeAutoScalingInstancesOutput{
			AutoScalingInstances: []*awsautoscaling.InstanceDetails{
				{
					AutoScalingGroupName: aws.String("test"),
				},
			},
		}, nil).
		Once()

	name, err := e.getAutoscalingGroupNameForInstanceID(instanceID)
	assert.NoError(t, err, "no error for good describe autoscaling instances response")
	assert.Equal(t, "test", *name)
}

func TestInstanceIDFromProviderID(t *testing.T) {
	providerID := "aws:///us-east-1a/i-0a2ade0106d44fd46"
	instanceID := instanceIDFromProviderID(providerID)
	assert.Equal(t, instanceID, "i-0a2ade0106d44fd46")
}

func TestGetRegion(t *testing.T) {
	expected := "us-east-1"

	os.Setenv("AWS_REGION", expected)
	defer os.Unsetenv("AWS_REGION")

	region := getRegion()
	assert.Equal(t, expected, region, "AWS region pulled from env if set")
}
