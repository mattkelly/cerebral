package app

import (
	//"os"
	"runtime"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/containership/cerebral/pkg/buildinfo"
	cerebral "github.com/containership/cerebral/pkg/client/clientset/versioned"
	cinformers "github.com/containership/cerebral/pkg/client/informers/externalversions"
	"github.com/containership/cerebral/pkg/controller"

	"github.com/containership/cluster-manager/pkg/log"
)

// Run creates the controller that reconciles the node count associated with
// an autoscaling group, with the autoscaling groups min and max nodes
func Run(stopCh <-chan struct{}) error {
	log.Info("Starting Containership AutoScaling Engine Controller...")
	log.Infof("Version: %s", buildinfo.String())
	log.Infof("Go Version: %s", runtime.Version())

	config := config.GetConfigOrDie()

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

	autoscalingGroupController := controller.NewAutoscalingGroupController(
		kubeclientset, kubeInformerFactory, cerebralclientset, cerebralInformerFactory)

	kubeInformerFactory.Start(stopCh)
	cerebralInformerFactory.Start(stopCh)

	if err = autoscalingGroupController.Run(1, stopCh); err != nil {
		log.Fatalf("Error running controller: %s", err.Error())
	}

	return nil
}
