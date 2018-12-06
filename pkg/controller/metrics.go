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
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelistersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"github.com/containership/cluster-manager/pkg/log"

	cerebralv1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	cerebral "github.com/containership/cerebral/pkg/client/clientset/versioned"
	cinformers "github.com/containership/cerebral/pkg/client/informers/externalversions"
	clisters "github.com/containership/cerebral/pkg/client/listers/cerebral.containership.io/v1alpha1"

	"github.com/pkg/errors"
)

const (
	metricsControllerName = "MetricsController"

	// this is the time delay between retries if a resource fails during sync
	metricsDelayBetweenRequeues = 30 * time.Second

	// number of times a Metrics will retry syncing
	metricsMaxRequeues = 10
)

// MetricsController is a controller that manages underlying metrics backend pollers
// based on AutoscalingGroups and AutoscalingPolicies.
// These pollers watch for alerts and request scaling if needed.
type MetricsController struct {
	kubeclientset     kubernetes.Interface
	cerebralclientset cerebral.Interface

	asgLister clisters.AutoscalingGroupLister
	asgSynced cache.InformerSynced

	aspLister clisters.AutoscalingPolicyLister
	aspSynced cache.InformerSynced

	// This controller does not listen on nodes, but it does need to keep a cache
	// of nodes in order to select nodes for AutoscalingGroups and pass them around
	nodeLister corelistersv1.NodeLister
	nodeSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface

	recorder record.EventRecorder

	// Key is ASG name
	pollManagers map[string]pollManager
}

// NewMetrics constructs a new Metrics controller
func NewMetrics(kubeclientset kubernetes.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	cerebralclientset cerebral.Interface,
	cInformerFactory cinformers.SharedInformerFactory) *MetricsController {
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(metricsDelayBetweenRequeues, metricsMaxRequeues)

	c := &MetricsController{
		kubeclientset:     kubeclientset,
		cerebralclientset: cerebralclientset,
		workqueue:         workqueue.NewNamedRateLimitingQueue(rateLimiter, metricsControllerName),
		pollManagers:      make(map[string]pollManager),
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{
		Interface: kubeclientset.CoreV1().Events(""),
	})
	c.recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{
		Component: metricsControllerName,
	})

	asgInformer := cInformerFactory.Cerebral().V1alpha1().AutoscalingGroups()
	aspInformer := cInformerFactory.Cerebral().V1alpha1().AutoscalingPolicies()
	nodeInformer := kubeInformerFactory.Core().V1().Nodes()

	log.Infof("%s: setting up event handlers", metricsControllerName)

	asgInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueAutoscalingGroup,
		UpdateFunc: func(old, new interface{}) {
			newASG := new.(*cerebralv1alpha1.AutoscalingGroup)
			oldASG := old.(*cerebralv1alpha1.AutoscalingGroup)
			// Generation need to be checked so that the ASG only gets enqueued if the
			// spec changes and ignores status update changes, as well as sync events
			if newASG.ResourceVersion == oldASG.ResourceVersion ||
				newASG.Generation == oldASG.Generation {
				return
			}
			c.enqueueAutoscalingGroup(new)
		},
		DeleteFunc: c.enqueueAutoscalingGroup,
	})

	aspInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueASGsForAutoscalingPolicy,
		UpdateFunc: func(old, new interface{}) {
			newASP := new.(*cerebralv1alpha1.AutoscalingPolicy)
			oldASP := old.(*cerebralv1alpha1.AutoscalingPolicy)
			if newASP.ResourceVersion == oldASP.ResourceVersion {
				return
			}
			c.enqueueASGsForAutoscalingPolicy(new)
		},
		DeleteFunc: c.enqueueASGsForAutoscalingPolicy,
	})

	c.asgLister = asgInformer.Lister()
	c.asgSynced = asgInformer.Informer().HasSynced

	c.aspLister = aspInformer.Lister()
	c.aspSynced = aspInformer.Informer().HasSynced

	c.nodeLister = nodeInformer.Lister()
	c.nodeSynced = nodeInformer.Informer().HasSynced

	return c
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *MetricsController) Run(numWorkers int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	log.Infof("Starting %s", metricsControllerName)

	if ok := cache.WaitForCacheSync(stopCh,
		c.asgSynced, c.aspSynced, c.nodeSynced); !ok {
		return fmt.Errorf("%s: failed to wait for caches to sync", metricsControllerName)
	}

	log.Infof("%s: starting workers", metricsControllerName)
	for i := 0; i < numWorkers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	log.Infof("%s: started workers", metricsControllerName)
	<-stopCh
	log.Infof("%s: shutting down workers", metricsControllerName)

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *MetricsController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem continually pops items off of the workqueue and handles
// them
func (c *MetricsController) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			log.Errorf("%s: expected string in workqueue but got %#v", metricsControllerName, obj)
			return nil
		}

		err := c.syncHandler(key)
		return c.handleErr(err, key)
	}(obj)

	if err != nil {
		log.Error(err)
		return true
	}

	return true
}

// handleErr drops the key from the workqueue if the error is nil or requeues
// it up to a maximum number of times
func (c *MetricsController) handleErr(err error, key interface{}) error {
	if err == nil {
		c.workqueue.Forget(key)
		return nil
	}

	if c.workqueue.NumRequeues(key) < maxRequeues {
		c.workqueue.AddRateLimited(key)
		return errors.Wrapf(err, "%s: error syncing AutoscalingGroup %q (has been requeued %d times)", metricsControllerName, key, c.workqueue.NumRequeues(key))
	}

	c.workqueue.Forget(key)
	log.Infof("%s: dropping AutoscalingGroup %q out of the queue: %v", metricsControllerName, key, err)
	return err
}

// enqueueAutoscalingGroup enqueues an AutoscalingGroup object.
func (c *MetricsController) enqueueAutoscalingGroup(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		log.Errorf("Error enqueueing AutoscalingGroup: %s", err)
		return
	}
	log.Debugf("%s: added %q to workqueue", metricsControllerName, key)
	c.workqueue.AddRateLimited(key)
}

// enqueueAutoscalingGroup enqueues all AutoscalingGroups for a given AutoscalingPolicy.
func (c *MetricsController) enqueueASGsForAutoscalingPolicy(obj interface{}) {
	asp, ok := obj.(*cerebralv1alpha1.AutoscalingPolicy)
	if !ok {
		log.Errorf("%s: unexpected type %T in call to enqueueASGsForAutoscalingPolicy", metricsControllerName, obj)
		return
	}

	log.Debugf("%s: finding ASGs to enqueue for event triggered on ASP %q", metricsControllerName, asp.ObjectMeta.Name)

	asgs, err := c.asgLister.List(labels.NewSelector())
	if err != nil {
		log.Errorf("%s: error getting AutoscalingGroups when node was enqueued: %s", metricsControllerName, err)
		return
	}

	log.Debugf("%s: no ASGs for ASP %q, ignoring", metricsControllerName, asp.ObjectMeta.Name)

	for _, asg := range asgs {
		for _, p := range asg.Spec.Policies {
			if p == asp.ObjectMeta.Name {
				log.Debugf("%s: enqueuing AutoscalingGroup %q for AutoscalingPolicy %q",
					metricsControllerName, asg.ObjectMeta.Name, asp.ObjectMeta.Name)
				c.enqueueAutoscalingGroup(asg)
			}
		}
	}
}

// syncHandler observes the current state of the system and reconciles the metric pollers
// associated with AutoscalingGroups and their referenced AutoscalingPolicies
func (c *MetricsController) syncHandler(key string) error {
	_, asgName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	asg, err := c.asgLister.Get(asgName)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			log.Infof("%s: AutoscalingGroup %s was deleted - cleaning up", metricsControllerName, asgName)
			c.cleanupPollManagerForASG(asgName)
			return nil
		}

		return err
	}

	// TODO obviously this can be easily optimized by updating the underlying types
	// instead of just shutting down, deleting, and recreating, but this works for now
	if _, ok := c.pollManagers[asgName]; ok {
		log.Debugf("%s: poll manager for %q already exists - it will be replaced", metricsControllerName, asgName)
		c.cleanupPollManagerForASG(asgName)
	}

	if asg.Spec.Suspended {
		log.Infof("%s: AutoscalingGroup %q is suspended - skipping", metricsControllerName, asgName)
		return nil
	}

	if len(asg.Spec.Policies) == 0 {
		log.Warnf("%s: AutoscalingGroup %q doesn't have any policies - skipping", metricsControllerName, asgName)
		return nil
	}

	// Get nodes associated with this AutoscalingGroup using the node selector
	selector := getNodesLabelSelector(asg.Spec.NodeSelector)
	nodes, err := c.nodeLister.List(selector)
	if err != nil {
		return errors.Wrapf(err, "listing nodes for AutoscalingGroup %q", asg.ObjectMeta.Name)
	}

	stopCh := make(chan struct{})
	mgr := newPollManager(asg, stopCh)

	// TODO it would be better if the manager itself created all of these, but it's easier to just
	// do it here for now :(
	for _, aspName := range asg.Spec.Policies {
		asp, err := c.aspLister.Get(aspName)
		if err != nil {
			if kubeerrors.IsNotFound(err) {
				log.Warnf("AutoscalingPolicy %q specified by AutoscalingGroup %q does not exist - skipping", aspName, asgName)
				continue
			}

			return err
		}

		aspCopy := asp.DeepCopy()
		p := newMetricPoller(*aspCopy, nodes)
		mgr.addPoller(p)
	}

	c.pollManagers[asgName] = mgr

	go func() {
		log.Debugf("Starting poll manager for AutoscalingGroup %q", asgName)
		if err := mgr.run(); err != nil {
			// Handle unexpected failures in the poller simply by requeueing the ASG so it tries again
			log.Errorf("Poll manager for AutoscalingGroup %q died: %s", asgName, err)
			c.enqueueAutoscalingGroup(asg)
		}
	}()

	return nil
}

// Close any metric pollers associated with this ASG and its ASPs and
// delete the poll manager from the map.
func (c *MetricsController) cleanupPollManagerForASG(asgName string) {
	var mgr pollManager
	var ok bool
	if mgr, ok = c.pollManagers[asgName]; !ok {
		// Nothing to do
		return
	}

	close(mgr.stopCh)

	delete(c.pollManagers, asgName)
}
