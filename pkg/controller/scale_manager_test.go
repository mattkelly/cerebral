package controller

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	"github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	"github.com/containership/cerebral/pkg/autoscaling"
	"github.com/containership/cerebral/pkg/autoscaling/mocks"
	"github.com/containership/cerebral/pkg/client/clientset/versioned/fake"
	informers "github.com/containership/cerebral/pkg/client/informers/externalversions"
)

var (
	noResyncPeriodFunc = func() time.Duration { return 0 }
	engineName         = "containership"
)

type fixture struct {
	t *testing.T

	client     *fake.Clientset
	kubeclient *k8sfake.Clientset
	// Objects to put in the store.
	asgListerObjects  []*v1alpha1.AutoscalingGroup
	nodeListerObjects []*corev1.Node
	// Objects from here preloaded into NewSimpleFake.
	kubeobjects []runtime.Object
	objects     []runtime.Object
}

func newFixture(t *testing.T) *fixture {
	return &fixture{
		t:           t,
		objects:     make([]runtime.Object, 0),
		kubeobjects: make([]runtime.Object, 0),
	}
}

var masterNodeTestLabels = map[string]string{
	"node-role.kubernetes.io/master": "",
}

func newBasicAutoscalingGroup() *v1alpha1.AutoscalingGroup {
	return newAutoscalingGroup("test", false, make(map[string]string, 0), 1, 3)
}

func newAutoscalingGroup(name string, suspended bool, labels map[string]string, min, max int) *v1alpha1.AutoscalingGroup {
	return &v1alpha1.AutoscalingGroup{
		TypeMeta: metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.AutoscalingGroupSpec{
			NodeSelector:   labels,
			Engine:         engineName,
			Suspended:      suspended,
			CooldownPeriod: 600,
			MinNodes:       min,
			MaxNodes:       max,
		},
	}
}

func newNode(name string, labels map[string]string) *corev1.Node {
	return &corev1.Node{
		TypeMeta: metav1.TypeMeta{APIVersion: v1alpha1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: corev1.NodeSpec{},
	}
}

func newScaleRequest(name string, direction scaleDirection, atype adjustmentType, ignoreCooldown bool) ScaleRequest {
	return ScaleRequest{
		asgName:         name,
		direction:       direction,
		adjustmentType:  atype,
		adjustmentValue: float64(3),
		ignoreCooldown:  ignoreCooldown,
		errCh:           make(chan error),
	}
}

func (f *fixture) newScaleManager() *ScaleManager {
	f.client = fake.NewSimpleClientset(f.objects...)
	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)

	i := informers.NewSharedInformerFactory(f.client, noResyncPeriodFunc())
	k8sI := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	c := NewScaleManager(f.kubeclient, k8sI, f.client, i)

	c.recorder = &record.FakeRecorder{}

	for _, a := range f.asgListerObjects {
		i.Cerebral().V1alpha1().AutoscalingGroups().Informer().GetIndexer().Add(a)
	}

	for _, n := range f.nodeListerObjects {
		k8sI.Core().V1().Nodes().Informer().GetIndexer().Add(n)
	}

	return c
}

func (f *fixture) run(req ScaleRequest) {
	f.runHandleScaleRequest(req, false)
}

func (f *fixture) runExpectError(req ScaleRequest) {
	f.runHandleScaleRequest(req, true)
}

func (f *fixture) runASGScaleRequestExpectError(asg *v1alpha1.AutoscalingGroup, req ScaleRequest) {
	f.runHandleScaleRequestForASG(asg, req, true, false)
}

func (f *fixture) runASGScaleRequestExpectScale(asg *v1alpha1.AutoscalingGroup, req ScaleRequest) {
	f.runHandleScaleRequestForASG(asg, req, false, true)
}

func (f *fixture) runASGScaleRequestExpectNoOp(asg *v1alpha1.AutoscalingGroup, req ScaleRequest) {
	f.runHandleScaleRequestForASG(asg, req, false, false)
}

// runHandleScaleRequest tests scale events to see if they error, or succeed as expected
func (f *fixture) runHandleScaleRequest(req ScaleRequest, expectError bool) {
	c := f.newScaleManager()

	err := c.handleScaleRequest(req)
	if !expectError {
		assert.NoError(f.t, err)
	}

	if expectError {
		assert.Error(f.t, err)
	}
}

// runHandleScaleRequestForASG lets us test handleScaleRequestForASG function to
// get a deeper look into seeing that scale events are triggered as expected. That
// means returning false when there was no scale event, and true when a scale
// event was triggered.
func (f *fixture) runHandleScaleRequestForASG(asg *v1alpha1.AutoscalingGroup, req ScaleRequest, expectError bool, expectScale bool) {
	c := f.newScaleManager()

	scaled, err := c.handleScaleRequestForASG(asg, req)
	if !expectError {
		assert.NoError(f.t, err)
		assert.Equalf(f.t, expectScale, scaled, "expected scaled to be %t but instead was %t", expectScale, scaled)
	}

	if expectError {
		assert.Error(f.t, err)
	}
}

func getKey(ag *v1alpha1.AutoscalingGroup, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(ag)
	assert.NoError(t, err, "Unexpected error getting key for foo %v: %v", ag.Name, err)

	return key
}

func TestNewScaleRequestWithNoASG(t *testing.T) {
	f := newFixture(t)

	req := newScaleRequest("asg-dne", scaleDirectionUp, adjustmentTypeAbsolute, true)

	f.run(req)
}

func TestNewScaleRequestWithoutEngine(t *testing.T) {
	f := newFixture(t)
	ag := newBasicAutoscalingGroup()

	f.asgListerObjects = append(f.asgListerObjects, ag)
	f.objects = append(f.objects, ag)

	req := newScaleRequest(getKey(ag, t), scaleDirectionUp, adjustmentTypeAbsolute, true)

	f.runExpectError(req)
}

func TestNewScaleRequestWithEngineWithoutNodes(t *testing.T) {
	f := newFixture(t)
	ag := newBasicAutoscalingGroup()

	f.asgListerObjects = append(f.asgListerObjects, ag)
	f.objects = append(f.objects, ag)

	req := newScaleRequest(getKey(ag, t), scaleDirectionUp, adjustmentTypeAbsolute, false)

	mockEngine := mocks.Engine{}
	// Return scaled and no error
	mockEngine.On("SetTargetNodeCount", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil).Once()

	autoscaling.Registry().Put(engineName, &mockEngine)
	defer autoscaling.Registry().Delete(engineName)

	f.run(req)
}

func TestNewScaleRequestWithNode(t *testing.T) {
	f := newFixture(t)
	ag := newBasicAutoscalingGroup()
	n := newNode("test", masterNodeTestLabels)

	f.asgListerObjects = append(f.asgListerObjects, ag)
	f.objects = append(f.objects, ag)

	f.nodeListerObjects = append(f.nodeListerObjects, n)
	f.kubeobjects = append(f.kubeobjects, n)

	req := newScaleRequest(getKey(ag, t), scaleDirectionUp, adjustmentTypeAbsolute, true)

	mockEngine := mocks.Engine{}
	// Return scaled and no error
	mockEngine.On("SetTargetNodeCount", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil).Once()

	autoscaling.Registry().Put(engineName, &mockEngine)
	defer autoscaling.Registry().Delete(engineName)

	f.run(req)
}

func TestASGSuspended(t *testing.T) {
	f := newFixture(t)
	ag := newAutoscalingGroup("test", true, masterNodeTestLabels, 1, 5)

	f.asgListerObjects = append(f.asgListerObjects, ag)
	f.objects = append(f.objects, ag)

	req := newScaleRequest(getKey(ag, t), scaleDirectionUp, adjustmentTypeAbsolute, true)

	autoscaling.Registry().Put(engineName, &mocks.Engine{})
	defer autoscaling.Registry().Delete(engineName)

	// testing that scale returns false, and that there is no error returned when
	// an ASG is suspended
	f.runASGScaleRequestExpectNoOp(ag, req)
}

func TestASGInCooldown(t *testing.T) {
	f := newFixture(t)
	ag := newBasicAutoscalingGroup()
	ag.Status = v1alpha1.AutoscalingGroupStatus{
		LastUpdatedAt: metav1.Now(),
	}

	f.asgListerObjects = append(f.asgListerObjects, ag)
	f.objects = append(f.objects, ag)

	req := newScaleRequest(getKey(ag, t), scaleDirectionDown, adjustmentTypeAbsolute, false)

	// testing that if ASG is in a cooldown period the handler returns false for
	// scaled and no error
	f.runASGScaleRequestExpectNoOp(ag, req)
}

func TestASGScalingWithMultipleNodes(t *testing.T) {
	f := newFixture(t)
	ag := newAutoscalingGroup("test", false, masterNodeTestLabels, 1, 1)
	n := newNode("test", masterNodeTestLabels)
	n2 := newNode("test2", masterNodeTestLabels)

	f.asgListerObjects = append(f.asgListerObjects, ag)
	f.objects = append(f.objects, ag)

	f.nodeListerObjects = append(f.nodeListerObjects, n)
	f.kubeobjects = append(f.kubeobjects, n)
	f.nodeListerObjects = append(f.nodeListerObjects, n2)
	f.kubeobjects = append(f.kubeobjects, n2)

	req := newScaleRequest(getKey(ag, t), scaleDirectionDown, adjustmentTypeAbsolute, false)

	mockEngine := mocks.Engine{}
	// Return scaled and no error
	mockEngine.On("SetTargetNodeCount", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil).Once()

	autoscaling.Registry().Put(engineName, &mockEngine)
	defer autoscaling.Registry().Delete(engineName)

	// testing that the ASG will return true and scale down when number of nodes is
	// greater than the max number of nodes defined in the ASG
	f.runASGScaleRequestExpectScale(ag, req)
}

func TestASGScalingWithMultipleNodesWhenCurrIsTarget(t *testing.T) {
	f := newFixture(t)
	ag := newAutoscalingGroup("test2", false, masterNodeTestLabels, 2, 2)
	n := newNode("test", masterNodeTestLabels)
	n2 := newNode("test2", masterNodeTestLabels)

	f.asgListerObjects = append(f.asgListerObjects, ag)
	f.objects = append(f.objects, ag)

	f.nodeListerObjects = append(f.nodeListerObjects, n)
	f.kubeobjects = append(f.kubeobjects, n)
	f.nodeListerObjects = append(f.nodeListerObjects, n2)
	f.kubeobjects = append(f.kubeobjects, n2)

	autoscaling.Registry().Put(engineName, &mocks.Engine{})
	defer autoscaling.Registry().Delete(engineName)

	agKey := getKey(ag, t)
	req := newScaleRequest(agKey, scaleDirectionDown, adjustmentTypeAbsolute, false)
	f.run(req)

	req = newScaleRequest(agKey, scaleDirectionUp, adjustmentTypeAbsolute, false)
	f.runASGScaleRequestExpectNoOp(ag, req)
}

func TestSetNodeCountNoOp(t *testing.T) {
	f := newFixture(t)
	ag := newAutoscalingGroup("test", false, masterNodeTestLabels, 1, 3)
	ag.Spec.ScalingStrategy = &v1alpha1.ScalingStrategy{
		ScaleUp: "do-not-scale",
	}

	f.asgListerObjects = append(f.asgListerObjects, ag)
	f.objects = append(f.objects, ag)

	req := newScaleRequest(getKey(ag, t), scaleDirectionUp, adjustmentTypeAbsolute, false)

	mockEngine := mocks.Engine{}
	// Return scaled and no error
	mockEngine.On("SetTargetNodeCount", mock.Anything, mock.Anything, mock.Anything).
		Return(false, nil).Once()

	autoscaling.Registry().Put(engineName, &mockEngine)
	defer autoscaling.Registry().Delete(engineName)

	// testing if the engine returns false that the scale manager returns
	// false, and that no scale event occurs
	f.runASGScaleRequestExpectNoOp(ag, req)
}

func TestSetNodeCountWithUnsupportedStrategy(t *testing.T) {
	f := newFixture(t)
	ag := newAutoscalingGroup("test", false, masterNodeTestLabels, 1, 3)
	ag.Spec.ScalingStrategy = &v1alpha1.ScalingStrategy{
		ScaleUp: "unsupported-strategy",
	}

	f.asgListerObjects = append(f.asgListerObjects, ag)
	f.objects = append(f.objects, ag)

	req := newScaleRequest(getKey(ag, t), scaleDirectionUp, adjustmentTypeAbsolute, false)

	mockEngine := mocks.Engine{}
	// Return scaled and no error
	mockEngine.On("SetTargetNodeCount", mock.Anything, mock.Anything, mock.Anything).
		Return(false, fmt.Errorf("engine returned error")).Once()

	autoscaling.Registry().Put(engineName, &mockEngine)
	defer autoscaling.Registry().Delete(engineName)

	// testing that if the engine returns an error the scale manager also returns
	// and error and that no scale event occurred.
	f.runASGScaleRequestExpectError(ag, req)
}

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
	{
		curr:            4,
		min:             1,
		max:             5,
		dir:             scaleDirectionDown,
		adjustmentType:  adjustmentTypePercent,
		adjustmentValue: 20,

		expected: 3,
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

func TestAdjustmentTypeFromString(t *testing.T) {
	a, err := adjustmentTypeFromString("absolute")
	assert.Nil(t, err)
	assert.Equal(t, adjustmentTypeAbsolute, a)

	a, err = adjustmentTypeFromString("percent")
	assert.Nil(t, err)
	assert.Equal(t, adjustmentTypePercent, a)

	_, err = adjustmentTypeFromString("doesnotexist")
	assert.Error(t, err)
}

func TestAdjustmentTypeToString(t *testing.T) {
	s := adjustmentTypeAbsolute.String()
	assert.Equal(t, "absolute", s)

	s = adjustmentTypePercent.String()
	assert.Equal(t, "percent", s)

	var adjustmentTypeDNE adjustmentType
	adjustmentTypeDNE = 3
	s = adjustmentTypeDNE.String()
	assert.Equal(t, "unknown", s)
}
