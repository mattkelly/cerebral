package controller

import (
	"fmt"
	"math"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"

	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelistersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	"github.com/containership/cluster-manager/pkg/log"

	cerebralv1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	"github.com/containership/cerebral/pkg/autoscaling"
	cerebral "github.com/containership/cerebral/pkg/client/clientset/versioned"
	cinformers "github.com/containership/cerebral/pkg/client/informers/externalversions"
	clisters "github.com/containership/cerebral/pkg/client/listers/cerebral.containership.io/v1alpha1"
	"github.com/containership/cerebral/pkg/events"
	"github.com/containership/cerebral/pkg/nodeutil"
)

type scaleDirection int

const (
	scaleDirectionUp scaleDirection = iota
	scaleDirectionDown
)

func (d scaleDirection) String() string {
	switch d {
	case scaleDirectionUp:
		return "up"
	case scaleDirectionDown:
		return "down"
	}

	return "unknown"
}

type adjustmentType int

const (
	adjustmentTypeAbsolute adjustmentType = iota
	adjustmentTypePercent
)

func (a adjustmentType) String() string {
	switch a {
	case adjustmentTypeAbsolute:
		return "absolute"
	case adjustmentTypePercent:
		return "percent"
	}

	return "unknown"
}

func adjustmentTypeFromString(s string) (adjustmentType, error) {
	switch s {
	case "absolute":
		return adjustmentTypeAbsolute, nil
	case "percent":
		return adjustmentTypePercent, nil
	}

	return 0, errors.Errorf("invalid adjustment type %q", s)
}

// ScaleManager manages incoming scale requests. It acts as the final stage
// before the actual engine interface, serializing requests to the engine and
// managing the AutoscalingGroup statuses to reflect cooldown state.
type ScaleManager struct {
	cerebralclientset cerebral.Interface

	asgLister clisters.AutoscalingGroupLister
	asgSynced cache.InformerSynced

	nodeLister corelistersv1.NodeLister
	nodeSynced cache.InformerSynced

	recorder record.EventRecorder

	scaleRequestCh chan ScaleRequest
}

// A ScaleRequest represents a request to the ScaleManager to perform a scaling
// operation.
type ScaleRequest struct {
	asgName         string
	direction       scaleDirection
	adjustmentType  adjustmentType
	adjustmentValue float64
	ignoreCooldown  bool

	// This channel is used for responding to the request so that the caller
	// may handle errors properly
	errCh chan error
}

const (
	scaleManagerName = "ScaleManager"
)

// NewScaleManager returns a new ScaleManager
func NewScaleManager(
	kubeclientset kubernetes.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	cerebralclientset cerebral.Interface,
	cInformerFactory cinformers.SharedInformerFactory) *ScaleManager {

	m := &ScaleManager{
		cerebralclientset: cerebralclientset,
		scaleRequestCh:    make(chan ScaleRequest),
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{
		Interface: kubeclientset.CoreV1().Events(""),
	})
	m.recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{
		Component: controllerName,
	})

	asgInformer := cInformerFactory.Cerebral().V1alpha1().AutoscalingGroups()
	nodeInformer := kubeInformerFactory.Core().V1().Nodes()

	m.asgLister = asgInformer.Lister()
	m.asgSynced = asgInformer.Informer().HasSynced

	m.nodeLister = nodeInformer.Lister()
	m.nodeSynced = nodeInformer.Informer().HasSynced

	return m
}

// ScaleRequestChan returns a channel that can be used to send scale requests
// to the ScaleManager
func (m *ScaleManager) ScaleRequestChan() chan<- ScaleRequest {
	return m.scaleRequestCh
}

// Run runs the ScaleManager. It should never return under normal conditions.
// It must respond to every request on the request's errCh, with the response being
// nil if no error occurred.
func (m *ScaleManager) Run(stopCh <-chan struct{}) error {
	if ok := cache.WaitForCacheSync(stopCh, m.asgSynced, m.nodeSynced); !ok {
		return errors.Errorf("%s: failed to wait for caches to sync", scaleManagerName)
	}

	for {
		select {
		case req := <-m.scaleRequestCh:
			if req.errCh == nil {
				// This will shut everything down - it should be a programming error
				// For all other errors we should actually return them in the channel
				return errors.New("received scale request without a response channel")
			}

			log.Debugf("%s: got scale request: %+v", scaleManagerName, req)

			req.errCh <- m.handleScaleRequest(req)

		case <-stopCh:
			log.Info("Shutting down scale manager")
			return nil
		}
	}
}

func (m *ScaleManager) handleScaleRequest(req ScaleRequest) error {
	asg, err := m.asgLister.Get(req.asgName)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			log.Infof("%s: AutoscalingGroup %q was deleted - ignoring scale request", scaleManagerName, req.asgName)
			return nil
		}

		return errors.Wrapf(err, "getting AutoscalingGroup %q to scale", req.asgName)
	}

	scaled, err := m.handleScaleRequestForASG(asg, req)
	if err != nil {
		return err
	}

	if !scaled {
		return nil
	}

	// TODO instead of just returning an error here, we should consider blocking further
	// scale requests for this ASG while we try to update the status
	err = m.updateAutoscalingGroupStatus(asg)
	if err != nil {
		return errors.Wrapf(err, "updating status for AutoscalingGroup %q", req.asgName)
	}

	return nil
}

func (m *ScaleManager) handleScaleRequestForASG(asg *cerebralv1alpha1.AutoscalingGroup, req ScaleRequest) (bool, error) {
	if asg.Spec.Suspended {
		// This should only really happen if there's an outstanding scale request
		// when an actor edits the CR to suspend it
		m.recorder.Event(asg, corev1.EventTypeNormal, events.ScaleIgnored, "AutoscalingGroup is suspended")
		return false, nil
	}

	if !req.ignoreCooldown && isCoolingDown(asg) {
		m.recorder.Event(asg, corev1.EventTypeNormal, events.ScaleIgnored, "AutoscalingGroup is cooling down")
		return false, nil
	}

	engine, err := autoscaling.Registry().Get(asg.Spec.Engine)
	if err != nil {
		return false, errors.Wrapf(err, "getting engine %q from registry", asg.Spec.Engine)
	}

	ns := nodeutil.GetNodesLabelSelector(asg.Spec.NodeSelector)
	nodes, err := m.nodeLister.List(ns)
	if err != nil {
		return false, errors.Wrapf(err, "listing nodes for AutoscalingGroup %q", req.asgName)
	}

	currNodeCount := len(nodes)
	targetNodeCount := calculateTargetNodeCount(currNodeCount, asg.Spec.MinNodes, asg.Spec.MaxNodes,
		req.direction, req.adjustmentType, req.adjustmentValue)

	if currNodeCount == targetNodeCount {
		// The scale operation would be a noop, so just ignore it but record
		// a warning event if this case is interesting
		if req.direction == scaleDirectionUp && targetNodeCount == asg.Spec.MaxNodes {
			m.recorder.Event(asg, corev1.EventTypeWarning, events.ScaleIgnored,
				fmt.Sprintf("Scale %s operation would exceed upper bound of %d nodes",
					req.direction.String(), asg.Spec.MaxNodes))
		} else if req.direction == scaleDirectionDown && targetNodeCount == asg.Spec.MinNodes {
			m.recorder.Event(asg, corev1.EventTypeWarning, events.ScaleIgnored,
				fmt.Sprintf("Scale %s operation would exceed lower bound of %d nodes",
					req.direction.String(), asg.Spec.MinNodes))
		}

		return false, nil
	}

	strategy := getAutoscalingGroupStrategy(req.direction, asg)

	scaled, err := engine.SetTargetNodeCount(asg.Spec.NodeSelector, targetNodeCount, strategy)
	if err != nil {
		m.recorder.Event(asg, corev1.EventTypeWarning, events.ScaleError,
			fmt.Sprintf("Failed to scale: %s", err))
		return false, err
	}

	if !scaled {
		return false, nil
	}

	if req.direction == scaleDirectionUp {
		m.recorder.Event(asg, corev1.EventTypeNormal, events.ScaledUp,
			fmt.Sprintf("Scaled up to %d nodes using strategy %q", targetNodeCount, strategy))
	} else {
		m.recorder.Event(asg, corev1.EventTypeNormal, events.ScaledDown,
			fmt.Sprintf("Scaled down to %d nodes using strategy %q", targetNodeCount, strategy))
	}

	return true, nil
}

func (m *ScaleManager) updateAutoscalingGroupStatus(asg *cerebralv1alpha1.AutoscalingGroup) error {
	asgCopy := asg.DeepCopy()
	asgCopy.Status.LastUpdatedAt = metav1.Now()
	_, err := m.cerebralclientset.CerebralV1alpha1().AutoscalingGroups().UpdateStatus(asgCopy)
	return err
}

func calculateTargetNodeCount(curr, min, max int,
	dir scaleDirection, adjustmentType adjustmentType, adjustmentValue float64) int {
	var result int

	switch adjustmentType {
	case adjustmentTypeAbsolute:
		// As documented, we truncate to an int since float makes no sense and
		// there's no way to validate via the subset of OpenAPI v3
		if dir == scaleDirectionUp {
			result = curr + int(adjustmentValue)
		} else {
			result = curr - int(adjustmentValue)
		}

	case adjustmentTypePercent:
		// Example: 25.5% should be specified as 25.5 in the CR, not 0.255
		adjustBy := float64(curr) * (0.01 * adjustmentValue)

		// As documented, take the ceiling of the result to avoid getting stuck
		if dir == scaleDirectionUp {
			result = int(float64(curr) + math.Ceil(adjustBy))
		} else {
			result = int(float64(curr) - math.Ceil(adjustBy))
		}
	}

	return fitWithinBounds(result, min, max)
}

// Fit a value within the given min and max bounds (inclusive),
// returning the value passed in if it's already within the bounds.
func fitWithinBounds(val, min, max int) int {
	if val < min {
		return min
	} else if val > max {
		return max
	}

	return val
}

func isCoolingDown(asg *cerebralv1alpha1.AutoscalingGroup) bool {
	if asg.Status.LastUpdatedAt.IsZero() {
		return false
	}

	return (nowFunc().Unix() - asg.Status.LastUpdatedAt.Unix()) <= int64(asg.Spec.CooldownPeriod)
}

// Given the scale direction, return the scaling strategy associated with it.
// If ScalingStrategy is not provided in the ASG spec, an empty string is returned
// and the engine is expected to handle defaulting (or erroring) as appropriate.
func getAutoscalingGroupStrategy(dir scaleDirection, asg *cerebralv1alpha1.AutoscalingGroup) string {
	if asg.Spec.ScalingStrategy == nil {
		return ""
	}

	// The individual strategies may still be unspecified (empty strings), and that's ok
	var strategy string
	if dir == scaleDirectionUp {
		strategy = asg.Spec.ScalingStrategy.ScaleUp
	} else {
		strategy = asg.Spec.ScalingStrategy.ScaleDown
	}

	return strategy
}
