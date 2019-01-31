package kubernetestest

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

// BuildNodeLister gets a node lister. Copies of the nodes are added to the cache;
// not the nodes themselves.
func BuildNodeLister(nodes []corev1.Node) corelistersv1.NodeLister {
	// We don't need anything related to the client or informer; we're simply
	// using this as an easy way to build a cache
	client := &fake.Clientset{}
	kubeInformerFactory := informers.NewSharedInformerFactory(client, 30*time.Second)
	informer := kubeInformerFactory.Core().V1().Nodes()

	for _, node := range nodes {
		// TODO why is DeepCopy() required here? Without it, each Add() duplicates
		// the first member added.
		err := informer.Informer().GetStore().Add(node.DeepCopy())
		if err != nil {
			// Should be a programming error
			panic(err)
		}
	}

	return informer.Lister()
}
