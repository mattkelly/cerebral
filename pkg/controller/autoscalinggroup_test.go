package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"

	cerebralv1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
)

type buildDesiredCountTest struct {
	name     string
	curr     int
	min      int
	max      int
	expected int
}

var desiredCountTests = []*buildDesiredCountTest{
	&buildDesiredCountTest{
		name:     "Current node count is with in min and max bounds, return current count",
		curr:     4,
		min:      3,
		max:      7,
		expected: 4,
	},
	&buildDesiredCountTest{
		name:     "Current node count is less than min, return min",
		curr:     1,
		min:      2,
		max:      3,
		expected: 2,
	},
	&buildDesiredCountTest{
		name:     "Current node count is greater than max, return max",
		curr:     7,
		min:      3,
		max:      5,
		expected: 5,
	},
	&buildDesiredCountTest{
		name:     "Current node count is equal to min, return current count",
		curr:     1,
		min:      1,
		max:      2,
		expected: 1,
	},
}

func TestGetDesiredNodeCount(t *testing.T) {
	for _, test := range desiredCountTests {
		desired := getDesiredNodeCount(test.curr, test.min, test.max)
		assert.Equal(t, test.expected, desired, test.name)
	}
}

type buildNodesLabelSelector struct {
	name           string
	labels         map[string]string
	expectedString string
}

var buildNodesLabelSelectorTests = []*buildNodesLabelSelector{
	&buildNodesLabelSelector{
		name: "check single label",
		labels: map[string]string{
			"key": "value",
		},
		expectedString: "key=value",
	},
	&buildNodesLabelSelector{
		name: "check multiple labels",
		labels: map[string]string{
			"key-1": "value-1",
			"key-2": "value-2",
		},
		expectedString: "key-1=value-1,key-2=value-2",
	},
	// TODO: are we supporting empty?
	&buildNodesLabelSelector{
		name:           "check empty labels",
		labels:         map[string]string{},
		expectedString: "",
	},
}

func TestGetNodesLabelSelector(t *testing.T) {
	for _, test := range buildNodesLabelSelectorTests {
		r := getNodesLabelSelector(test.labels)
		assert.Equal(t, test.expectedString, r.String(), test.name)
	}
}

type buildFindNodesASG struct {
	name       string
	nodeLabels map[string]string
	asgs       []*cerebralv1alpha1.AutoScalingGroup
	expected   *cerebralv1alpha1.AutoScalingGroup
}

var singleLabel = map[string]string{
	"test": "one",
}

var singleASG = &cerebralv1alpha1.AutoScalingGroup{
	Spec: cerebralv1alpha1.AutoScalingGroupSpec{
		NodeSelector: singleLabel,
	},
}

var multipleLabels = map[string]string{
	"test":   "one",
	"second": "test",
}

var multipleASG = &cerebralv1alpha1.AutoScalingGroup{
	Spec: cerebralv1alpha1.AutoScalingGroupSpec{
		NodeSelector: multipleLabels,
	},
}

var nonMatchingASG = &cerebralv1alpha1.AutoScalingGroup{
	Spec: cerebralv1alpha1.AutoScalingGroupSpec{
		NodeSelector: map[string]string{
			"something": "thatdoesntmatch",
		},
	},
}

var emptyASG = &cerebralv1alpha1.AutoScalingGroup{
	Spec: cerebralv1alpha1.AutoScalingGroupSpec{
		NodeSelector: map[string]string{},
	},
}

var findNodesASGsTests = []*buildFindNodesASG{
	&buildFindNodesASG{
		name:       "test one label asg",
		nodeLabels: singleLabel,
		asgs: []*cerebralv1alpha1.AutoScalingGroup{
			singleASG,
		},
		expected: singleASG,
	},
	&buildFindNodesASG{
		name:       "multiple labels asg",
		nodeLabels: multipleLabels,
		asgs: []*cerebralv1alpha1.AutoScalingGroup{
			multipleASG,
			nonMatchingASG,
		},
		expected: multipleASG,
	},
	&buildFindNodesASG{
		name:       "test non match asg",
		nodeLabels: singleLabel,
		asgs: []*cerebralv1alpha1.AutoScalingGroup{
			multipleASG,
		},
		expected: nil,
	},
	&buildFindNodesASG{
		name:       "test empty asg",
		nodeLabels: singleLabel,
		asgs: []*cerebralv1alpha1.AutoScalingGroup{
			emptyASG,
			multipleASG,
		},
		expected: emptyASG,
	},
}

func TestFindNodeASG(t *testing.T) {
	for _, test := range findNodesASGsTests {
		asg := findNodeASG(test.nodeLabels, test.asgs)
		assert.Equal(t, test.expected, asg, test.name)
	}
}
