package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"

	cerebralv1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	"github.com/containership/cerebral/pkg/nodeutil"
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
		name:     "Current node count is within min and max bounds, return current count",
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
	&buildNodesLabelSelector{
		name:           "check empty labels",
		labels:         map[string]string{},
		expectedString: "",
	},
}

func TestGetNodesLabelSelector(t *testing.T) {
	for _, test := range buildNodesLabelSelectorTests {
		r := nodeutil.GetNodesLabelSelector(test.labels)
		assert.Equal(t, test.expectedString, r.String(), test.name)
	}
}

type buildFindNodesAG struct {
	name       string
	nodeLabels map[string]string
	ags        []*cerebralv1alpha1.AutoscalingGroup
	expected   []*cerebralv1alpha1.AutoscalingGroup
}

var singleLabel = map[string]string{
	"test": "one",
}

var singleAG = &cerebralv1alpha1.AutoscalingGroup{
	Spec: cerebralv1alpha1.AutoscalingGroupSpec{
		NodeSelector: singleLabel,
	},
}

var multipleLabels = map[string]string{
	"test":   "one",
	"second": "test",
}

var multipleLabelsAG = &cerebralv1alpha1.AutoscalingGroup{
	Spec: cerebralv1alpha1.AutoscalingGroupSpec{
		NodeSelector: multipleLabels,
	},
}

var nonMatchingAG = &cerebralv1alpha1.AutoscalingGroup{
	Spec: cerebralv1alpha1.AutoscalingGroupSpec{
		NodeSelector: map[string]string{
			"something": "thatdoesntmatch",
		},
	},
}

var emptyAG = &cerebralv1alpha1.AutoscalingGroup{
	Spec: cerebralv1alpha1.AutoscalingGroupSpec{
		NodeSelector: map[string]string{},
	},
}

var findNodesAGsTests = []*buildFindNodesAG{
	&buildFindNodesAG{
		name:       "test one label ag",
		nodeLabels: singleLabel,
		ags: []*cerebralv1alpha1.AutoscalingGroup{
			singleAG,
		},
		expected: []*cerebralv1alpha1.AutoscalingGroup{
			singleAG,
		},
	},
	&buildFindNodesAG{
		name:       "multiple labels ag",
		nodeLabels: multipleLabels,
		ags: []*cerebralv1alpha1.AutoscalingGroup{
			multipleLabelsAG,
			nonMatchingAG,
		},
		expected: []*cerebralv1alpha1.AutoscalingGroup{
			multipleLabelsAG,
		},
	},
	&buildFindNodesAG{
		name:       "test non match ag",
		nodeLabels: singleLabel,
		ags: []*cerebralv1alpha1.AutoscalingGroup{
			multipleLabelsAG,
		},
		expected: []*cerebralv1alpha1.AutoscalingGroup{},
	},
	&buildFindNodesAG{
		name:       "test empty ag",
		nodeLabels: singleLabel,
		ags: []*cerebralv1alpha1.AutoscalingGroup{
			emptyAG,
			multipleLabelsAG,
		},
		expected: []*cerebralv1alpha1.AutoscalingGroup{
			emptyAG,
		},
	}, &buildFindNodesAG{
		name:       "test multiple ag selection",
		nodeLabels: multipleLabels,
		ags: []*cerebralv1alpha1.AutoscalingGroup{
			emptyAG,
			singleAG,
			multipleLabelsAG,
		},
		expected: []*cerebralv1alpha1.AutoscalingGroup{
			emptyAG,
			singleAG,
			multipleLabelsAG,
		},
	},
}

func TestFindAGsMatchingNodeLabels(t *testing.T) {
	for _, test := range findNodesAGsTests {
		ag := findAGsMatchingNodeLabels(test.nodeLabels, test.ags)
		assert.Equal(t, test.expected, ag, test.name)
	}
}

func TestIsScaleUpEvent(t *testing.T) {
	// isScaleUpEvent(curr, desired)
	scale := isScaleUpEvent(3, 5) //true
	assert.True(t, scale)

	scale = isScaleUpEvent(7, 1) //false
	assert.False(t, scale)

	scale = isScaleUpEvent(5, 5) //false
	assert.False(t, scale)
}

func TestGetAutoscalingGroupStrategy(t *testing.T) {
	upStrategy := "custom-up"
	downStrategy := "custom-down"
	ag := &cerebralv1alpha1.AutoscalingGroup{
		Spec: cerebralv1alpha1.AutoscalingGroupSpec{
			ScalingStrategy: cerebralv1alpha1.ScalingStrategy{
				ScaleUp:   upStrategy,
				ScaleDown: downStrategy,
			},
		},
	}

	strategy := getAutoscalingGroupStrategy(true, ag.Spec) // "custom-up"
	assert.Equal(t, upStrategy, strategy)

	strategy = getAutoscalingGroupStrategy(false, ag.Spec) // "custom-down"
	assert.Equal(t, downStrategy, strategy)

	strategy = getAutoscalingGroupStrategy(false, cerebralv1alpha1.AutoscalingGroupSpec{}) // "random"
	assert.Equal(t, defaultAutoscalingStrategy, strategy)
}
