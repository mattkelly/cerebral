package controller

import (
	"fmt"
	"time"

	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	corev1 "k8s.io/api/core/v1"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelistersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/containership/cluster-manager/pkg/log"

	cerebralv1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	"github.com/containership/cerebral/pkg/autoscalingengine"
	cerebral "github.com/containership/cerebral/pkg/client/clientset/versioned"
	cerebralscheme "github.com/containership/cerebral/pkg/client/clientset/versioned/scheme"
	cinformers "github.com/containership/cerebral/pkg/client/informers/externalversions"
	clisters "github.com/containership/cerebral/pkg/client/listers/cerebral.containership.io/v1alpha1"
	"github.com/containership/cerebral/pkg/events"

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
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewAutoscalingGroupController returns a new controller to watch
// that the nodes associated with autoscaling groups stay between the min and max specified
func NewAutoscalingGroupController(kubeclientset kubernetes.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	cerebralclientset cerebral.Interface,
	cInformerFactory cinformers.SharedInformerFactory) *AutoscalingGroupController {
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(delayBetweenRequeues, maxRequeues)

	agc := &AutoscalingGroupController{
		kubeclientset:     kubeclientset,
		cerebralclientset: cerebralclientset,
		workqueue:         workqueue.NewNamedRateLimitingQueue(rateLimiter, controllerName),
	}

	cerebralscheme.AddToScheme(scheme.Scheme)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{
		Interface: kubeclientset.CoreV1().Events(""),
	})
	agc.recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{
		Component: controllerName,
	})

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
	log.Info("added to workqueue ", key)
	agc.workqueue.AddRateLimited(key)
}

// getDesiredNodeCount returns curr if curr is between the min and max bounds,
// if its below the bounds it will return min, if it's above the bounds it will
// return max.
func getDesiredNodeCount(curr, min, max int) int {
	if curr < min {
		return min
	} else if curr > max {
		return max
	}

	return curr
}

func isScaleUpEvent(curr, desired int) bool {
	return curr < desired
}

func getAutoscalingGroupStrategy(scaleUp bool, ag cerebralv1alpha1.AutoscalingGroupSpec) string {
	var strategy string
	if scaleUp {
		strategy = ag.ScalingStrategy.ScaleUp
	} else {
		strategy = ag.ScalingStrategy.ScaleDown
	}

	if strategy == "" {
		// TODO: update this to reference the value provided by the associated AutoscalingEngine.
		strategy = defaultAutoscalingStrategy
	}

	return strategy
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

	engineName := autoscalingGroup.Spec.Engine
	if !autoscalingengine.Registry().IsRegistered(engineName) {
		return errors.Errorf("the AutoscalingEngine specified for the AutoscalingGroup '%s' is not registered", autoscalingGroup.Name)
	}

	ns := getNodesLabelSelector(autoscalingGroup.Spec.NodeSelector)
	// get nodes associated with autoscaling group using the node selector
	nodes, _ := agc.nodeLister.List(ns)
	numNodes := len(nodes)
	log.Infof("Current number of nodes in autoscaling group '%s' : %d", autoscalingGroup.Name, numNodes)

	desired := getDesiredNodeCount(numNodes, autoscalingGroup.Spec.MinNodes, autoscalingGroup.Spec.MaxNodes)
	// If number of nodes is within threshold then nothing needs to change this
	// and this can noop and should just return
	if numNodes == desired {
		return nil
	}

	engine, err := autoscalingengine.Registry().Get(engineName)
	if err != nil {
		return err
	}

	scaledUp := isScaleUpEvent(numNodes, desired)
	strategy := getAutoscalingGroupStrategy(scaledUp, autoscalingGroup.Spec)
	scaled, err := engine.SetTargetNodeCount(autoscalingGroup.Spec.NodeSelector, desired, strategy)
	if err != nil {
		return err
	}

	if !scaled {
		return nil
	}

	if scaledUp {
		agc.recorder.Event(autoscalingGroup, corev1.EventTypeNormal, events.ScaledUp, fmt.Sprintf("Autoscaling group %s node count (%d) was not within min and max bounds and was scaled up to %d", autoscalingGroup.Name, numNodes, desired))
	} else {
		agc.recorder.Event(autoscalingGroup, corev1.EventTypeNormal, events.ScaledDown, fmt.Sprintf("Autoscaling group %s node count (%d) was not within min and max bounds and was scaled down to %d", autoscalingGroup.Name, numNodes, desired))
	}

	// TODO: rethink how to handle this error
	// https://github.com/containership/cerebral/issues/10
	return agc.updateAutoscalingGroupStatus(autoscalingGroup)
}

func (agc *AutoscalingGroupController) updateAutoscalingGroupStatus(autoscalingGroup *cerebralv1alpha1.AutoscalingGroup) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	agCopy := autoscalingGroup.DeepCopy()
	agCopy.Status.LastUpdatedAt = time.Now().Unix()
	_, err := agc.cerebralclientset.CerebralV1alpha1().AutoscalingGroups().UpdateStatus(agCopy)
	return err
}

// getNodesLabelSelector creates a selector object from the passed in labels map
func getNodesLabelSelector(labelsMap map[string]string) labels.Selector {
	selector := labels.NewSelector()
	for key, value := range labelsMap {
		l, _ := labels.NewRequirement(key, selection.Equals, []string{value})
		selector = selector.Add(*l)
	}

	return selector
}

// findNodesAGs goes through each autoscaling group and checks to see if the AG
// nodeSelector matches the node labels passed into the function returning all
// AGs that match
func findAGsMatchingNodeLabels(nodeLabels map[string]string, ags []*cerebralv1alpha1.AutoscalingGroup) []*cerebralv1alpha1.AutoscalingGroup {
	matchingags := make([]*cerebralv1alpha1.AutoscalingGroup, 0)

	for _, autoscalingGroup := range ags {
		// create selector object from nodeSelector of AG
		agselectors := getNodesLabelSelector(autoscalingGroup.Spec.NodeSelector)

		// check to see if the nodeSelector labels match the node labels that
		// were passed in
		if agselectors.Matches(labels.Set(nodeLabels)) {
			matchingags = append(matchingags, autoscalingGroup)
		}
	}

	return matchingags
}
