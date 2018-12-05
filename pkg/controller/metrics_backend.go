package controller

import (
	"fmt"
	"time"

	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
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

	"github.com/containership/cerebral/pkg/metrics"
	"github.com/containership/cerebral/pkg/metrics/backends/prometheus"

	"github.com/pkg/errors"
)

const (
	metricsBackendControllerName = "MetricsBackendController"

	// this is the time delay between retries if a resource fails during sync
	metricsBackendDelayBetweenRequeues = 30 * time.Second

	// number of times a MetricsBackend will retry syncing
	metricsBackendMaxRequeues = 10
)

// MetricsBackendController reconciles MetricsBackends with a local registry of
// instantiated backend clients.
type MetricsBackendController struct {
	kubeclientset     kubernetes.Interface
	cerebralclientset cerebral.Interface

	metricsBackendLister clisters.MetricsBackendLister
	metricsBackendSynced cache.InformerSynced

	// The Prometheus backend requires pod and node listers in order to gather
	// node exporter info, so just hold it here so it can be initialized with
	// everything else to avoid weirdness
	nodeLister corelistersv1.NodeLister
	nodeSynced cache.InformerSynced

	podLister corelistersv1.PodLister
	podSynced cache.InformerSynced

	workqueue workqueue.RateLimitingInterface

	recorder record.EventRecorder
}

// NewMetricsBackend constructs a new MetricsBackend
func NewMetricsBackend(kubeclientset kubernetes.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	cerebralclientset cerebral.Interface,
	cInformerFactory cinformers.SharedInformerFactory) *MetricsBackendController {
	rateLimiter := workqueue.NewItemExponentialFailureRateLimiter(metricsBackendDelayBetweenRequeues, metricsBackendMaxRequeues)

	c := &MetricsBackendController{
		kubeclientset:     kubeclientset,
		cerebralclientset: cerebralclientset,
		workqueue:         workqueue.NewNamedRateLimitingQueue(rateLimiter, metricsBackendControllerName),
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{
		Interface: kubeclientset.CoreV1().Events(""),
	})
	c.recorder = eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{
		Component: metricsBackendControllerName,
	})

	metricsBackendInformer := cInformerFactory.Cerebral().V1alpha1().MetricsBackends()

	nodeInformer := kubeInformerFactory.Core().V1().Nodes()
	podInformer := kubeInformerFactory.Core().V1().Pods()

	log.Infof("%s: setting up event handlers", metricsBackendControllerName)

	metricsBackendInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueMetricsBackend,
		UpdateFunc: func(old, new interface{}) {
			// We want to ignore periodic resyncs
			newBackend := new.(*cerebralv1alpha1.MetricsBackend)
			oldBackend := old.(*cerebralv1alpha1.MetricsBackend)
			if newBackend.ResourceVersion == oldBackend.ResourceVersion {
				return
			}

			c.enqueueMetricsBackend(new)
		},
		DeleteFunc: c.enqueueMetricsBackend,
	})

	c.metricsBackendLister = metricsBackendInformer.Lister()
	c.metricsBackendSynced = metricsBackendInformer.Informer().HasSynced

	c.nodeLister = nodeInformer.Lister()
	c.nodeSynced = nodeInformer.Informer().HasSynced

	c.podLister = podInformer.Lister()
	c.podSynced = podInformer.Informer().HasSynced

	return c
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *MetricsBackendController) Run(numWorkers int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	log.Infof("Starting %s", metricsBackendControllerName)

	if ok := cache.WaitForCacheSync(stopCh, c.metricsBackendSynced, c.nodeSynced, c.podSynced); !ok {
		// If this channel is unable to wait for caches to sync we stop
		// all controllers
		return errors.Errorf("%s: failed to wait for caches to sync", metricsBackendControllerName)
	}

	log.Infof("%s: starting workers", metricsBackendControllerName)
	// Launch numWorkers amount of workers to process resources
	for i := 0; i < numWorkers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	log.Infof("%s: started workers", metricsBackendControllerName)
	<-stopCh
	log.Infof("%s: shutting down workers", metricsBackendControllerName)

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *MetricsBackendController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem continually pops items off of the workqueue and handles
// them
func (c *MetricsBackendController) processNextWorkItem() bool {
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
			log.Errorf("%s: expected string in workqueue but got %#v", metricsBackendControllerName, obj)
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
func (c *MetricsBackendController) handleErr(err error, key interface{}) error {
	if err == nil {
		c.workqueue.Forget(key)
		return nil
	}

	if c.workqueue.NumRequeues(key) < maxRequeues {
		c.workqueue.AddRateLimited(key)
		return errors.Wrapf(err, "error syncing MetricsBackend %q (has been requeued %d times)", key, c.workqueue.NumRequeues(key))
	}

	c.workqueue.Forget(key)
	log.Infof("Dropping MetricsBackend %q out of the queue: %v", key, err)
	return err
}

// enqueueMetricsBackend enqueues a MetricsBackend object
func (c *MetricsBackendController) enqueueMetricsBackend(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		log.Error("Error enqueueing MetricsBackend: %s", err)
		return
	}
	log.Debugf("%s: added %q to workqueue ", metricsBackendControllerName, key)
	c.workqueue.AddRateLimited(key)
}

// syncHandler reconciles MetricsBackends being synced with a local cache
// of instantiated backend clients.
func (c *MetricsBackendController) syncHandler(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	backend, err := c.metricsBackendLister.Get(name)
	if err != nil {
		if kubeerrors.IsNotFound(err) {
			// Must've been deleted, so let's delete it from the registry
			metrics.Registry().Delete(name)
			return nil
		}

		return err
	}

	// If the instantiated metrics backend already exists, let's just clean it up
	// and replace it instead of trying to update it. By deleting it here, we'll
	// cause anyone using the backend to see that it's no longer around and thus
	// backoff into a retry loop.
	if _, err := metrics.Registry().Get(name); err == nil {
		log.Infof("%s: backend for %q already exists - it will be replaced", metricsBackendControllerName, name)
		metrics.Registry().Delete(name)
	}

	log.Infof("Instantiating backend client for MetricsBackend %q", name)
	client, err := c.instantiateBackend(backend)
	if err != nil {
		return errors.Wrapf(err, "instantiating backend client for MetricsBackend %q", name)
	}
	metrics.Registry().Put(name, client)
	log.Infof("Backend %q instantiated successfully", name)

	return nil
}

// insantiateBackend instantiates a new backend for the given MetricsBackend.
// It should be the only function that knows how to instantiate a particular backend type.
func (c *MetricsBackendController) instantiateBackend(backend *cerebralv1alpha1.MetricsBackend) (metrics.Backend, error) {
	switch backend.Spec.Type {
	case "prometheus":
		var address string
		var ok bool
		address, ok = backend.Spec.Configuration["address"]
		if !ok {
			return nil, errors.New("Prometheus backend requires address in configuration")
		}

		return prometheus.NewClient(address, c.nodeLister, c.podLister)

	default:
		return nil, errors.Errorf("unknown backend type %q", backend.Spec.Type)
	}
}
