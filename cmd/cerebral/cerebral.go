package main

import (
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cerebralv1alpha1 "github.com/containership/cerebral/pkg/apis/cerebral.containership.io/v1alpha1"
	"github.com/containership/cerebral/pkg/autoscalingengine"
	"github.com/containership/cerebral/pkg/autoscalingengine/containership"
	"github.com/containership/cerebral/pkg/buildinfo"
	cerebral "github.com/containership/cerebral/pkg/client/clientset/versioned"
	cinformers "github.com/containership/cerebral/pkg/client/informers/externalversions"
	"github.com/containership/cerebral/pkg/controller"

	"github.com/containership/cluster-manager/pkg/log"
)

func main() {
	log.Info("Starting Cerebral...")
	log.Infof("Version: %s", buildinfo.String())
	log.Infof("Go Version: %s", runtime.Version())

	config, err := determineConfig()
	if err != nil {
		log.Fatal(err)
	}

	kubeclientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes clientset: %+v", err)
	}

	cerebralclientset, err := cerebral.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Cerebral clientset: %+v", err)
	}

	kubeInformerFactory := informers.NewSharedInformerFactory(kubeclientset, 30*time.Second)
	cerebralInformerFactory := cinformers.NewSharedInformerFactory(cerebralclientset, 30*time.Second)

	registerContainershipEngineOrDie(cerebralclientset)

	autoscalingGroupController := controller.NewAutoscalingGroupController(
		kubeclientset, kubeInformerFactory, cerebralclientset, cerebralInformerFactory)

	metricsBackendController := controller.NewMetricsBackend(
		kubeclientset, kubeInformerFactory, cerebralclientset, cerebralInformerFactory)

	stopCh := make(chan struct{})
	kubeInformerFactory.Start(stopCh)
	cerebralInformerFactory.Start(stopCh)

	go func() {
		if err := autoscalingGroupController.Run(1, stopCh); err != nil {
			log.Fatalf("Error running AutoscalingGroup controller: %s", err.Error())
		}
	}()

	go func() {
		if err := metricsBackendController.Run(1, stopCh); err != nil {
			log.Fatalf("Error running MetricsBackend controller: %s", err.Error())
		}
	}()

	<-stopCh
	log.Fatal("There was an error while running the controllers")
}

// determineConfig determines if we are running in a cluster or outside
// and gets the appropriate configuration to talk with Kubernetes.
func determineConfig() (*rest.Config, error) {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	var config *rest.Config
	var err error

	// determine whether to use in cluster config or out of cluster config
	// if kubeconfigPath is not specified, default to in cluster config
	// otherwise, use out of cluster config
	if kubeconfigPath == "" {
		log.Info("Using in cluster k8s config")
		config, err = rest.InClusterConfig()
	} else {
		log.Info("Using out of cluster k8s config: ", kubeconfigPath)

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}

	if err != nil {
		return nil, errors.Wrap(err, "determine Kubernetes config failed")
	}

	return config, nil
}

// registerContainershipEngineOrDie assumes a few things...
// 1. It assumes that the containership autoscaling engine CR exists, if not cerebral fails
// 2. Only engines of type containership will be registered
// 3. Finally if the containership autoscaling engine is unable to be created it
//    assumes cerebral can't/shouldn't be used and fails.
func registerContainershipEngineOrDie(cerebralclientset *cerebral.Clientset) {
	engineCreated := false

	enginecrs, err := cerebralclientset.Cerebral().AutoscalingEngines().List(metav1.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}

	for _, enginecr := range enginecrs.Items {
		// only register engines of type containership
		switch strings.ToLower(enginecr.Spec.Type) {
		case "containership":
			engineCreated, err = initializeAndRegisterContainershipEngine(enginecr)
			if err != nil {
				log.Fatalf("Failed to create Containership autoscaling engine: %+v", err)
			}

		default:
			log.Infof("Autoscaling Engine of type '%s' is unable to be registered", enginecr.Spec.Type)
		}

	}

	if !engineCreated {
		log.Fatalf("No AutoscalingEngine of type containership found. Failed to create an autoscaling engine")
	}
}

func initializeAndRegisterContainershipEngine(enginecr cerebralv1alpha1.AutoscalingEngine) (bool, error) {
	engine, err := containership.NewAutoscalingEngine(enginecr)
	if err != nil {
		return false, err
	}

	autoscalingengine.Registry().Put(engine)
	return true, nil
}
