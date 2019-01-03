package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"

	cerebralv1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	"github.com/containership/cerebral/pkg/nodeutil"
)

type buildNodesLabelSelector struct {
	name           string
	labels         map[string]string
	expectedString string
}

var buildNodesLabelSelectorTests = []*buildNodesLabelSelector{
	{
		name: "check single label",
		labels: map[string]string{
			"key": "value",
		},
		expectedString: "key=value",
	},
	{
		name: "check multiple labels",
		labels: map[string]string{
			"key-1": "value-1",
			"key-2": "value-2",
		},
		expectedString: "key-1=value-1,key-2=value-2",
	},
	{
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
	{
		name:       "test one label ag",
		nodeLabels: singleLabel,
		ags: []*cerebralv1alpha1.AutoscalingGroup{
			singleAG,
		},
		expected: []*cerebralv1alpha1.AutoscalingGroup{
			singleAG,
		},
	},
	{
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
	{
		name:       "test non match ag",
		nodeLabels: singleLabel,
		ags: []*cerebralv1alpha1.AutoscalingGroup{
			multipleLabelsAG,
		},
		expected: []*cerebralv1alpha1.AutoscalingGroup{},
	},
	{
		name:       "test empty ag",
		nodeLabels: singleLabel,
		ags: []*cerebralv1alpha1.AutoscalingGroup{
			emptyAG,
			multipleLabelsAG,
		},
		expected: []*cerebralv1alpha1.AutoscalingGroup{
			emptyAG,
		},
	},
	{
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

func TestGetAutoscalingGroupStrategy(t *testing.T) {
	upStrategy := "custom-up"
	downStrategy := "custom-down"
	ag := &cerebralv1alpha1.AutoscalingGroup{
		Spec: cerebralv1alpha1.AutoscalingGroupSpec{
			ScalingStrategy: &cerebralv1alpha1.ScalingStrategy{
				ScaleUp:   upStrategy,
				ScaleDown: downStrategy,
			},
		},
	}

	strategy := getAutoscalingGroupStrategy(scaleDirectionUp, ag) // "custom-up"
	assert.Equal(t, upStrategy, strategy)

	strategy = getAutoscalingGroupStrategy(scaleDirectionDown, ag) // "custom-down"
	assert.Equal(t, downStrategy, strategy)

	strategy = getAutoscalingGroupStrategy(scaleDirectionDown, &cerebralv1alpha1.AutoscalingGroup{})
	assert.Equal(t, "", strategy, "no down strategy is ok (engine defaults, not this)")

	strategy = getAutoscalingGroupStrategy(scaleDirectionUp, &cerebralv1alpha1.AutoscalingGroup{})
	assert.Equal(t, "", strategy, "no up strategy is ok (engine defaults, not this)")
}

type scaleDeltaTest struct {
	curr int
	min  int
	max  int

	expectedDelta int
	expectedDir   scaleDirection

	message string
}

var scaleDeltaTests = []scaleDeltaTest{
	{
		curr: 0,
		min:  1,
		max:  1,

		expectedDelta: 1,
		expectedDir:   scaleDirectionUp,

		message: "scale up from 0 to min",
	},
	{
		curr: 1,
		min:  1,
		max:  1,

		expectedDelta: 0,

		message: "curr == min == max is a noop",
	},
	{
		curr: 1,
		min:  3,
		max:  5,

		expectedDelta: 2,
		expectedDir:   scaleDirectionUp,

		message: "scale up to min",
	},
	{
		curr: 3,
		min:  3,
		max:  5,

		expectedDelta: 0,

		message: "bounds check is inclusive (curr == min is a noop)",
	},
	{
		curr: 5,
		min:  3,
		max:  5,

		expectedDelta: 0,

		message: "bounds check is inclusive (curr == max is a noop)",
	},
	{
		curr: 0,
		min:  0,
		max:  0,

		expectedDelta: 0,

		message: "curr == min == max == 0 is a noop",
	},
	{
		curr: 7,
		min:  3,
		max:  5,

		expectedDelta: 2,
		expectedDir:   scaleDirectionDown,

		message: "scale down to max",
	},
	{
		curr: 7,
		min:  0,
		max:  0,

		expectedDelta: 7,
		expectedDir:   scaleDirectionDown,

		message: "scale down to max of 0",
	},
}

func TestDetermineScaleDeltaAndDirection(t *testing.T) {
	for _, test := range scaleDeltaTests {
		delta, dir := determineScaleDeltaAndDirection(test.curr, test.min, test.max)
		assert.Equal(t, test.expectedDelta, delta, test.message)
		if test.expectedDelta != 0 {
			assert.Equal(t, test.expectedDir, dir, test.message)
		}
	}
}

func TestAbs(t *testing.T) {
	assert.Equal(t, 0, abs(0))
	assert.Equal(t, 1, abs(1))
	assert.Equal(t, 1, abs(-1))
	assert.Equal(t, 50, abs(50))
	assert.Equal(t, 50, abs(-50))
}
