package main

import (
	"os"
	"runtime"
	"time"

	"github.com/pkg/errors"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/containership/cerebral/pkg/autoscalingengine"
	"github.com/containership/cerebral/pkg/autoscalingengine/containership"
	"github.com/containership/cerebral/pkg/buildinfo"
	cerebral "github.com/containership/cerebral/pkg/client/clientset/versioned"
	cinformers "github.com/containership/cerebral/pkg/client/informers/externalversions"
	"github.com/containership/cerebral/pkg/controller"

	// Register the autoscaling engine that needs to be initialized
	_ "github.com/containership/cerebral/pkg/autoscalingengine/containership"
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

	ae := autoscalingengine.New()
	ae.Register(containership.NewAutoscalingEngine())

	autoscalingGroupController := controller.NewAutoscalingGroupController(
		kubeclientset, kubeInformerFactory, cerebralclientset, cerebralInformerFactory, ae)

	stopCh := make(chan struct{})
	kubeInformerFactory.Start(stopCh)
	cerebralInformerFactory.Start(stopCh)

	if err = autoscalingGroupController.Run(1, stopCh); err != nil {
		log.Fatalf("Error running controller: %s", err.Error())
	}
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
