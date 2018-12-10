package controller

import (
	"fmt"
	"time"

	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	corev1 "k8s.io/api/core/v1"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelistersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/containership/cluster-manager/pkg/log"

	cerebralv1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	cerebral "github.com/containership/cerebral/pkg/client/clientset/versioned"
	cinformers "github.com/containership/cerebral/pkg/client/informers/externalversions"
	clisters "github.com/containership/cerebral/pkg/client/listers/cerebral.containership.io/v1alpha1"
	"github.com/containership/cerebral/pkg/nodeutil"

	"github.com/pkg/errors"
)

const (
	// controllerName is the name that the controller is registered as
	controllerName = "AutoscalingGroupController"

	// this is the time delay between retries if a resource fails during sync
	delayBetweenRequeues = 30 * time.Second

	// number of times an autoscaling group will retry syncing
	maxRequeues = 5

	// the default scaling strategy to pass the autoscaling engine if one is not user defined
	defaultAutoscalingStrategy = "random"
)

// AutoscalingGroupController is a controller for scaling an autoscaling
// group based on min and max
type AutoscalingGroupController struct {
	kubeclientset     kubernetes.Interface
	cerebralclientset cerebral.Interface

	nodeLister  corelistersv1.NodeLister
	nodesSynced cache.InformerSynced

	agLister clisters.AutoscalingGroupLister
	agSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface

	scaleRequestCh chan<- ScaleRequest
}

// NewAutoscalingGroupController returns a new controller to watch
// that the nodes associated with autoscaling groups stay between the min and max specified
func NewAutoscalingGroupController(kubeclientset kubernetes.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	cerebralclientset cerebral.Interface,
	cInformerFactory cinformers.SharedInformerFactory,
	scaleRequestCh chan<- ScaleRequest) *AutoscalingGroupController {
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(delayBetweenRequeues, maxRequeues)

	agc := &AutoscalingGroupController{
		kubeclientset:     kubeclientset,
		cerebralclientset: cerebralclientset,
		workqueue:         workqueue.NewNamedRateLimitingQueue(rateLimiter, controllerName),
		scaleRequestCh:    scaleRequestCh,
	}

	nodeInformer := kubeInformerFactory.Core().V1().Nodes()
	agInformer := cInformerFactory.Cerebral().V1alpha1().AutoscalingGroups()

	log.Info("Setting up event handlers")

	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    agc.enqueueAGForNode,
		DeleteFunc: agc.enqueueAGForNode,
	})

	agInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: agc.enqueueAutoscalingGroup,
		UpdateFunc: func(old, new interface{}) {
			newAG := new.(*cerebralv1alpha1.AutoscalingGroup)
			oldAG := old.(*cerebralv1alpha1.AutoscalingGroup)
			// Generation need to be checked so that the AG only gets enqueued if the
			// spec changes and ignores status update changes, as well as sync events
			if newAG.ResourceVersion == oldAG.ResourceVersion ||
				newAG.Generation == oldAG.Generation {
				return
			}
			agc.enqueueAutoscalingGroup(new)
		},
	})

	agc.nodeLister = nodeInformer.Lister()
	agc.nodesSynced = nodeInformer.Informer().HasSynced

	agc.agLister = agInformer.Lister()
	agc.agSynced = agInformer.Informer().HasSynced

	return agc
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (agc *AutoscalingGroupController) Run(numWorkers int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer agc.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	log.Info("Starting controller")

	if ok := cache.WaitForCacheSync(stopCh,
		agc.nodesSynced,
		agc.agSynced); !ok {
		// If this channel is unable to wait for caches to sync we return an error
		return fmt.Errorf("failed to wait for caches to sync")
	}

	log.Info("Starting workers")
	// Launch numWorkers amount of workers to process resources
	for i := 0; i < numWorkers; i++ {
		go wait.Until(agc.runWorker, time.Second, stopCh)
	}

	log.Info("Started workers")
	<-stopCh
	log.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (agc *AutoscalingGroupController) runWorker() {
	for agc.processNextWorkItem() {
	}
}

// processNextWorkItem continually pops items off of the workqueue and handles
// them
func (agc *AutoscalingGroupController) processNextWorkItem() bool {
	obj, shutdown := agc.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer agc.workqueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			agc.workqueue.Forget(obj)
			log.Errorf("expected string in workqueue but got %#v", obj)
			return nil
		}

		err := agc.syncHandler(key)
		return agc.handleErr(err, key)
	}(obj)

	if err != nil {
		log.Error(err)
		return true
	}

	return true
}

// handleErr drops the key from the workqueue if the error is nil or requeues
// it up to a maximum number of times
func (agc *AutoscalingGroupController) handleErr(err error, key interface{}) error {
	if err == nil {
		agc.workqueue.Forget(key)
		return nil
	}

	if agc.workqueue.NumRequeues(key) < maxRequeues {
		agc.workqueue.AddRateLimited(key)
		log.Error(err)
		return errors.Wrapf(err, "error syncing autoscaling group %q (has been requeued %d times)", key, agc.workqueue.NumRequeues(key))
	}

	agc.workqueue.Forget(key)
	log.Infof("Dropping autoscaling group %q out of the queue: %v", key, err)
	return err
}

// enqueueAGForNode enqueues the AG that matches the enqueued node's labels
func (agc *AutoscalingGroupController) enqueueAGForNode(obj interface{}) {
	node, _ := obj.(*corev1.Node)
	l := node.Labels

	// get all autoscaling groups
	ags, err := agc.agLister.List(labels.NewSelector())
	if err != nil {
		log.Error("Error getting autoscaling groups when node was enqueued", err)
		return
	}

	matchingAGs := findAGsMatchingNodeLabels(l, ags)
	// If the node is not associated with any autoscaling groups
	// we don't need to worry about doing anything and can just return
	if len(ags) == 0 {
		return
	}

	for _, ag := range matchingAGs {
		agc.enqueueAutoscalingGroup(ag)
	}
}

// enqueueAutoscalingGroup enqueues an autoscalinggroup object.
func (agc *AutoscalingGroupController) enqueueAutoscalingGroup(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		log.Error("Error enqueueing autoscaling group", err)
		return
	}
	log.Debugf("%s: added %q to workqueue", controllerName, key)
	agc.workqueue.AddRateLimited(key)
}

// syncHandler surveys the state of the systems, checking to see that the current
// number of nodes selected by an autoscaling groups node selector is within the
// autoscaling groups bounds for min and max and taking actions to reconcile if not
func (agc *AutoscalingGroupController) syncHandler(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(errors.Errorf("invalid resource key: %s", key))
		return nil
	}

	autoscalingGroup, err := agc.agLister.Get(name)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			return nil
		}

		return err
	}

	if autoscalingGroup.Spec.Suspended {
		log.Infof("Autoscaling Group '%s' was queued but it is currently suspended.", autoscalingGroup.Name)
		return nil
	}

	ns := nodeutil.GetNodesLabelSelector(autoscalingGroup.Spec.NodeSelector)
	// get nodes associated with autoscaling group using the node selector
	nodes, err := agc.nodeLister.List(ns)
	if err != nil {
		return errors.Wrapf(err, "listing nodes for AutoscalingGroup %s", autoscalingGroup.Name)
	}

	numNodes := len(nodes)
	log.Infof("Current number of nodes in autoscaling group '%s' : %d", autoscalingGroup.Name, numNodes)

	delta, dir := determineScaleDeltaAndDirection(numNodes, autoscalingGroup.Spec.MinNodes, autoscalingGroup.Spec.MaxNodes)
	if delta == 0 {
		log.Debugf("%s: AutoscalingGroup %s is within bounds - ignoring", controllerName, autoscalingGroup.Name)
		return nil
	}

	// We'll record an actual event in the scale manager, but log here at least
	log.Infof("AutoscalingGroup %s node count (%d) was not within min and max bounds and is requesting scale %s by %d",
		autoscalingGroup.Name, numNodes, dir.String(), delta)

	// This controller does not respect the cooldown because we want to immediately
	// reconcile situations where we're outside of the expected bounds
	// (This can happen due to an actor editing the ASG CR)
	errCh := make(chan error)
	agc.scaleRequestCh <- ScaleRequest{
		asgName:         autoscalingGroup.Name,
		direction:       dir,
		adjustmentType:  adjustmentTypeAbsolute,
		adjustmentValue: float64(delta),
		ignoreCooldown:  true,
		errCh:           errCh,
	}

	err = <-errCh
	if err != nil {
		// If a scale request fails, just return an error so the relevant ASG can be re-enqueued
		return errors.Wrap(err, "requesting scale manager to scale")
	}

	return nil
}

// findAGsMatchingNodeLabels goes through each autoscaling group and checks to see if the AG
// nodeSelector matches the node labels passed into the function returning all
// AGs that match
func findAGsMatchingNodeLabels(nodeLabels map[string]string, ags []*cerebralv1alpha1.AutoscalingGroup) []*cerebralv1alpha1.AutoscalingGroup {
	matchingags := make([]*cerebralv1alpha1.AutoscalingGroup, 0)

	for _, autoscalingGroup := range ags {
		// create selector object from nodeSelector of AG
		agselectors := nodeutil.GetNodesLabelSelector(autoscalingGroup.Spec.NodeSelector)

		// check to see if the nodeSelector labels match the node labels that
		// were passed in
		if agselectors.Matches(labels.Set(nodeLabels)) {
			matchingags = append(matchingags, autoscalingGroup)
		}
	}

	return matchingags
}

// determineScaleDeltaAndDirection determines, based on the current node count and min/max
// of the ASG, if we should scale to be back within the bounds and if so, in which direction.
// If a delta of 0 is returned, then we do not need to scale and the direction returned
// should be ignored.
// If a non-zero delta is returned, then we should scale in the returned direction.
func determineScaleDeltaAndDirection(currNodeCount, minNodes, maxNodes int) (int, scaleDirection) {
	target := fitWithinBounds(currNodeCount, minNodes, maxNodes)
	if target == currNodeCount {
		// We're already within the bounds, so nothing to do
		return 0, scaleDirectionDown
	}

	delta := currNodeCount - target
	dir := scaleDirectionDown
	if delta < 0 {
		dir = scaleDirectionUp
		delta = abs(delta)
	}

	return delta, dir
}

// abs returns the absolute value of an int
func abs(val int) int {
	if val < 0 {
		return -val
	}

	return val
}
