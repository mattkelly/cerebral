package controller

import (
	"fmt"
	"time"

	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

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

	"github.com/containership/cerebral/pkg/autoscaling"
	"github.com/containership/cerebral/pkg/autoscaling/engines/containership"
	"github.com/containership/cerebral/pkg/autoscaling/engines/digitalocean"

	"github.com/pkg/errors"
)

const (
	autoscalingEngineControllerName = "AutoscalingEngineController"

	// this is the time delay between retries if a resource fails during sync
	autoscalingEngineDelayBetweenRequeues = 30 * time.Second

	// number of times an AutoscalingEngine will retry syncing
	autoscalingEngineMaxRequeues = 10
)

// AutoscalingEngineController reconciles AutoscalingEngines with a local registry of
// instantiated engine clients.
type AutoscalingEngineController struct {
	kubeclientset     kubernetes.Interface
	cerebralclientset cerebral.Interface

	autoscalingEngineLister clisters.AutoscalingEngineLister
	autoscalingEngineSynced cache.InformerSynced

	nodeLister corelistersv1.NodeLister
	nodeSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface
}

// NewAutoscalingEngine constructs a new AutoscalingEngine
func NewAutoscalingEngine(kubeclientset kubernetes.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	cerebralclientset cerebral.Interface,
	cInformerFactory cinformers.SharedInformerFactory) *AutoscalingEngineController {
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(autoscalingEngineDelayBetweenRequeues, autoscalingEngineMaxRequeues)

	c := &AutoscalingEngineController{
		kubeclientset:     kubeclientset,
		cerebralclientset: cerebralclientset,
		workqueue:         workqueue.NewNamedRateLimitingQueue(rateLimiter, autoscalingEngineControllerName),
	}

	autoscalingEngineInformer := cInformerFactory.Cerebral().V1alpha1().AutoscalingEngines()
	nodeInformer := kubeInformerFactory.Core().V1().Nodes()

	log.Infof("%s: setting up event handlers", autoscalingEngineControllerName)

	autoscalingEngineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueAutoscalingEngine,
		UpdateFunc: func(old, new interface{}) {
			// We want to ignore periodic resyncs
			newEngine := new.(*cerebralv1alpha1.AutoscalingEngine)
			oldEngine := old.(*cerebralv1alpha1.AutoscalingEngine)
			if newEngine.ResourceVersion == oldEngine.ResourceVersion {
				return
			}

			c.enqueueAutoscalingEngine(new)
		},
		DeleteFunc: c.enqueueAutoscalingEngine,
	})

	c.autoscalingEngineLister = autoscalingEngineInformer.Lister()
	c.autoscalingEngineSynced = autoscalingEngineInformer.Informer().HasSynced

	c.nodeLister = nodeInformer.Lister()
	c.nodeSynced = nodeInformer.Informer().HasSynced

	return c
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *AutoscalingEngineController) Run(numWorkers int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	log.Infof("Starting %s", autoscalingEngineControllerName)

	if ok := cache.WaitForCacheSync(stopCh, c.autoscalingEngineSynced, c.nodeSynced); !ok {
		return errors.Errorf("%s: failed to wait for caches to sync", autoscalingEngineControllerName)
	}

	log.Infof("%s: starting workers", autoscalingEngineControllerName)
	// Launch numWorkers amount of workers to process resources
	for i := 0; i < numWorkers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	log.Infof("%s: started workers", autoscalingEngineControllerName)
	<-stopCh
	log.Infof("%s: shutting down workers", autoscalingEngineControllerName)

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *AutoscalingEngineController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem continually pops items off of the workqueue and handles
// them
func (c *AutoscalingEngineController) processNextWorkItem() bool {
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
			log.Errorf("%s: expected string in workqueue but got %#v", autoscalingEngineControllerName, obj)
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
func (c *AutoscalingEngineController) handleErr(err error, key interface{}) error {
	if err == nil {
		c.workqueue.Forget(key)
		return nil
	}

	if c.workqueue.NumRequeues(key) < maxRequeues {
		c.workqueue.AddRateLimited(key)
		return errors.Wrapf(err, "error syncing AutoscalingEngine %q (has been requeued %d times)", key, c.workqueue.NumRequeues(key))
	}

	c.workqueue.Forget(key)
	log.Infof("Dropping AutoscalingEngine %q out of the queue: %v", key, err)
	return err
}

// enqueueAutoscalingEngine enqueues an AutoscalingEngine object
func (c *AutoscalingEngineController) enqueueAutoscalingEngine(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		log.Error("Error enqueueing AutoscalingEngine: %s", err)
		return
	}

	log.Debugf("%s: added %q to workqueue ", autoscalingEngineControllerName, key)
	c.workqueue.AddRateLimited(key)
}

// syncHandler reconciles AutoscalingEngines being synced with a local cache
// of instantiated engine clients.
func (c *AutoscalingEngineController) syncHandler(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	engine, err := c.autoscalingEngineLister.Get(name)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			// Must've been deleted, so let's delete it from the registry
			autoscaling.Registry().Delete(name)
			return nil
		}

		return err
	}

	// If the instantiated autoscaling engine already exists, let's just clean it up
	// and replace it instead of trying to update it. By deleting it here, we'll
	// cause anyone using the engine to see that it's no longer around and thus
	// backoff into a retry loop.
	if _, err := autoscaling.Registry().Get(name); err == nil {
		log.Infof("%s: engine for %q already exists - it will be replaced", autoscalingEngineControllerName, name)
		autoscaling.Registry().Delete(name)
	}

	log.Infof("Instantiating engine client for AutoscalingEngine %q", name)

	client, err := instantiateEngine(engine, c.nodeLister)
	if err != nil {
		return errors.Wrapf(err, "instantiating engine client for AutoscalingEngine %q", name)
	}

	autoscaling.Registry().Put(name, client)
	log.Infof("Engine %q instantiated successfully", name)

	return nil
}

// instantiateEngine instantiates a new engine for the given AutoscalingEngine.
// It should be the only function that knows how to instantiate a particular engine type.
func instantiateEngine(engine *cerebralv1alpha1.AutoscalingEngine, nodeLister corelistersv1.NodeLister) (autoscaling.Engine, error) {
	switch engine.Spec.Type {
	case "containership":
		// Ignore defensive checks on engine property values since validation happens
		// upon new client creation. We're explicitly not copying the name and configuration
		// here since it is assumed that NewClient will not modify the parameters
		cae, err := containership.NewClient(engine.Name, engine.Spec.Configuration, nodeLister)
		if err != nil {
			return nil, errors.Wrapf(err, "constructing new containership engine %q", engine.Name)
		}

		return cae, nil

	case "digitalocean":
		do, err := digitalocean.NewClient(engine.Name, engine.Spec.Configuration, nodeLister)
		if err != nil {
			return nil, errors.Wrapf(err, "constructing new digitalocean engine %q", engine.Name)
		}

		return do, nil

	default:
		return nil, errors.Errorf("unknown engine type %q", engine.Spec.Type)
	}
}
