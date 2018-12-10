package controller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
)

type calculateTargetNodeCountTest struct {
	curr            int
	min             int
	max             int
	dir             scaleDirection
	adjustmentType  adjustmentType
	adjustmentValue float64

	expected int
	message  string
}

var calculateTargetNodeCountTests = []calculateTargetNodeCountTest{
	{
		curr:            2,
		min:             1,
		max:             5,
		dir:             scaleDirectionUp,
		adjustmentType:  adjustmentTypeAbsolute,
		adjustmentValue: 1,

		expected: 3,
		message:  "absolute with whole number",
	},
	{
		curr:            2,
		min:             1,
		max:             5,
		dir:             scaleDirectionUp,
		adjustmentType:  adjustmentTypeAbsolute,
		adjustmentValue: 1.75,

		expected: 3,
		message:  "absolute with fractional number takes floor of adjustment value",
	},
	{
		curr:            2,
		min:             1,
		max:             5,
		dir:             scaleDirectionDown,
		adjustmentType:  adjustmentTypeAbsolute,
		adjustmentValue: 4.75,

		expected: 1,
		message:  "absolute would scale below min",
	},
	{
		curr:            2,
		min:             1,
		max:             5,
		dir:             scaleDirectionDown,
		adjustmentType:  adjustmentTypeAbsolute,
		adjustmentValue: 1.75,

		expected: 1,
		message:  "absolute scales to min",
	},
	{
		curr:            4,
		min:             1,
		max:             5,
		dir:             scaleDirectionUp,
		adjustmentType:  adjustmentTypeAbsolute,
		adjustmentValue: 2.75,

		expected: 5,
		message:  "absolute would scale above max",
	},
	{
		curr:            4,
		min:             1,
		max:             5,
		dir:             scaleDirectionUp,
		adjustmentType:  adjustmentTypeAbsolute,
		adjustmentValue: 1.75,

		expected: 5,
		message:  "absolute scales to max",
	},
	{
		curr:            2,
		min:             1,
		max:             5,
		dir:             scaleDirectionDown,
		adjustmentType:  adjustmentTypePercent,
		adjustmentValue: 400,

		expected: 1,
		message:  "percent would scale below min",
	},
	{
		curr:            2,
		min:             1,
		max:             5,
		dir:             scaleDirectionDown,
		adjustmentType:  adjustmentTypePercent,
		adjustmentValue: 100,

		expected: 1,
		message:  "percent scales to min",
	},
	{
		curr:            4,
		min:             1,
		max:             5,
		dir:             scaleDirectionUp,
		adjustmentType:  adjustmentTypePercent,
		adjustmentValue: 100,

		expected: 5,
		message:  "percent would scale above max",
	},
	{
		curr:            2,
		min:             1,
		max:             4,
		dir:             scaleDirectionUp,
		adjustmentType:  adjustmentTypePercent,
		adjustmentValue: 100,

		expected: 4,
		message:  "percent scales to max",
	},
	{
		curr:            1,
		min:             1,
		max:             5,
		dir:             scaleDirectionUp,
		adjustmentType:  adjustmentTypePercent,
		adjustmentValue: 25,

		expected: 2,
		message:  "percent takes ceiling",
	},
}

type fitWithinBoundsTest struct {
	name     string
	val      int
	min      int
	max      int
	expected int
}

var fitWithinBoundsTests = []fitWithinBoundsTest{
	fitWithinBoundsTest{
		name:     "val is within min and max bounds, return val",
		val:      4,
		min:      3,
		max:      7,
		expected: 4,
	},
	fitWithinBoundsTest{
		name:     "val is less than min, return min",
		val:      1,
		min:      2,
		max:      3,
		expected: 2,
	},
	fitWithinBoundsTest{
		name:     "val is greater than max, return max",
		val:      7,
		min:      3,
		max:      5,
		expected: 5,
	},
	fitWithinBoundsTest{
		name:     "val is equal to min, return val",
		val:      1,
		min:      1,
		max:      2,
		expected: 1,
	},
}

func TestCalculateSetTargetNodeCount(t *testing.T) {
	for _, test := range calculateTargetNodeCountTests {
		result := calculateTargetNodeCount(test.curr, test.min, test.max,
			test.dir, test.adjustmentType, test.adjustmentValue)
		assert.Equal(t, test.expected, result, "%+v", test.message)
	}
}

func TestFitWithinBounds(t *testing.T) {
	for _, test := range fitWithinBoundsTests {
		val := fitWithinBounds(test.val, test.min, test.max)
		assert.Equal(t, test.expected, val, test.name)
	}
}

func TestIsCoolingDown(t *testing.T) {
	defer resetTime()

	asg := &v1alpha1.AutoscalingGroup{
		Spec: v1alpha1.AutoscalingGroupSpec{
			CooldownPeriod: 5,
		},
		Status: v1alpha1.AutoscalingGroupStatus{},
	}

	// Special case: a scale has never been triggered and thus LastUpdatedAt is unset
	setTime(0) // doesn't matter but just so it's a known value
	assert.False(t, isCoolingDown(asg), "unset LastUpdatedAt means not cooling down")

	now := time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)
	asg.Status.LastUpdatedAt = metav1.Time{
		Time: now,
	}

	setTime(now.Add(time.Second * 2).Unix())
	assert.True(t, isCoolingDown(asg), "cooldown period is inclusive at beginning: (now == lastUpdatedAt) --> in cooldown)")

	setTime(now.Add(time.Second * 4).Unix())
	assert.True(t, isCoolingDown(asg), "cooldown period in middle")

	setTime(now.Add(time.Second * 5).Unix())
	assert.True(t, isCoolingDown(asg), "cooldown period is inclusive at end")

	setTime(now.Add(time.Second * 8).Unix())
	assert.False(t, isCoolingDown(asg), "done cooling down")
}

type handleScaleRequestTest struct {
	asg *v1alpha1.AutoscalingGroup
	req ScaleRequest
}

func HandleScaleRequestForASG(t *testing.T) {
	mgr := ScaleManager{}

	asg := &v1alpha1.AutoscalingGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1alpha1.AutoscalingGroupSpec{
			Suspended: true,
		},
	}

	req := ScaleRequest{
		asgName:   asg.Name,
		direction: scaleDirectionUp,
	}

	scaled, err := mgr.handleScaleRequestForASG(asg, req)
	assert.False(t, scaled, "no action taken if suspended")
	assert.Nil(t, err, "no error if suspended")

	asg.Spec.Suspended = false

	scaled, err = mgr.handleScaleRequestForASG(asg, req)
	assert.False(t, scaled, "no action taken if suspended")
	assert.Nil(t, err, "no error if suspended")
}
