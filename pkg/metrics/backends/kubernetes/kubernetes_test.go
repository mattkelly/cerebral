package kubernetes

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	corelistersv1 "k8s.io/client-go/listers/core/v1"
)

var (
	nodeAllocatable = corev1.ResourceList{
		corev1.ResourceCPU:              *resource.NewQuantity(2, resource.DecimalSI),
		corev1.ResourceMemory:           *resource.NewQuantity(1024, resource.DecimalSI),
		corev1.ResourceEphemeralStorage: *resource.NewQuantity(4096, resource.DecimalSI),
		corev1.ResourcePods:             *resource.NewQuantity(2, resource.DecimalSI),
	}
)

var (
	node0 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-0",
		},
		Status: corev1.NodeStatus{
			Allocatable: nodeAllocatable,
		},
	}
	node1 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
		},
		Status: corev1.NodeStatus{
			Allocatable: nodeAllocatable,
		},
	}
	node2 = &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-2",
		},
		Status: corev1.NodeStatus{
			Allocatable: nodeAllocatable,
		},
	}

	pod0 = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-0",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: node0.ObjectMeta.Name,
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:              *resource.NewQuantity(1, resource.DecimalSI),
							corev1.ResourceMemory:           *resource.NewQuantity(256, resource.DecimalSI),
							corev1.ResourceEphemeralStorage: *resource.NewQuantity(1024, resource.DecimalSI),
						},
					},
				},
			},
		},
	}

	pod1 = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-1",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: node1.ObjectMeta.Name,
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:              *resource.NewQuantity(2, resource.DecimalSI),
							corev1.ResourceMemory:           *resource.NewQuantity(512, resource.DecimalSI),
							corev1.ResourceEphemeralStorage: *resource.NewQuantity(2048, resource.DecimalSI),
						},
					},
				},
			},
		},
	}

	pod2 = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-2",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: node1.ObjectMeta.Name,
		},
	}
)

var backend = Backend{
	nodeLister: buildNodeLister([]corev1.Node{*node0, *node1, *node2}),
	podLister:  buildPodLister([]corev1.Pod{*pod0, *pod1, *pod2}),
}

func TestGetValue(t *testing.T) {
	_, err := backend.GetValue("cpu_percent_allocation", nil, nil)
	assert.NoError(t, err, "successfully get cpu allocation metric")

	_, err = backend.GetValue("memory_percent_allocation", nil, nil)
	assert.NoError(t, err, "successfully get memory allocation metric")

	_, err = backend.GetValue("ephemeral_storage_percent_allocation", nil, nil)
	assert.NoError(t, err, "successfully get ephemeral storage allocation metric")

	_, err = backend.GetValue("pod_percent_allocation", nil, nil)
	assert.NoError(t, err, "successfully get pod allocation metric")

	_, err = backend.GetValue("not a valid metric", nil, nil)
	assert.Error(t, err, "unknown metric requested")
}

func TestGetPodsOnNodes(t *testing.T) {
	var emptyNodeList = []*corev1.Node{}
	var singleNodeList = []*corev1.Node{node0}
	var multipleNodeList = []*corev1.Node{node0, node1, node2}

	pods, _ := backend.getPodsOnNodes(nil)
	assert.Empty(t, pods, "nil node list returns empty array")

	pods, _ = backend.getPodsOnNodes(emptyNodeList)
	assert.Empty(t, pods, "empty node list returns empty array")

	pods, _ = backend.getPodsOnNodes(singleNodeList)
	assert.Equal(t, 1, len(pods), "returns single result")

	pods, _ = backend.getPodsOnNodes(multipleNodeList)
	assert.Equal(t, 3, len(pods), "returns three results")
}

func TestCalculateCPUAllocationPercentage(t *testing.T) {
	var podList = []*corev1.Pod{pod0, pod1, pod2}
	var nodeList = []*corev1.Node{node0, node1, node2}

	percentage := backend.calculateCPUAllocationPercentage(podList, nodeList)
	assert.Equal(t, float64(50), percentage, "returns correct allocation percentage")
}

func TestCalculateMemoryAllocationPercentage(t *testing.T) {
	var podList = []*corev1.Pod{pod0, pod1, pod2}
	var nodeList = []*corev1.Node{node0, node1, node2}

	percentage := backend.calculateMemoryAllocationPercentage(podList, nodeList)
	assert.Equal(t, float64(25), percentage, "returns correct allocation percentage")
}

func TestCalculateEphemeralStorageAllocationPercentage(t *testing.T) {
	var podList = []*corev1.Pod{pod0, pod1, pod2}
	var nodeList = []*corev1.Node{node0, node1, node2}

	percentage := backend.calculateEphemeralStorageAllocationPercentage(podList, nodeList)
	assert.Equal(t, float64(25), percentage, "returns correct allocation percentage")
}

func TestCalculatePodAllocationPercentage(t *testing.T) {
	var podList = []*corev1.Pod{pod0, pod1, pod2}
	var nodeList = []*corev1.Node{node0, node1, node2}

	percentage := backend.calculatePodAllocationPercentage(podList, nodeList)
	assert.Equal(t, float64(50), percentage, "returns correct allocation percentage")
}

// Get a node lister. Copies of the nodes are added to the cache; not the nodes themselves.
func buildNodeLister(nodes []corev1.Node) corelistersv1.NodeLister {
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

// Get a pod lister. Copies of the pods are added to the cache; not the pods themselves.
func buildPodLister(pods []corev1.Pod) corelistersv1.PodLister {
	// We don't need anything related to the client or informer; we're simply
	// using this as an easy way to build a cache
	client := &fake.Clientset{}
	kubeInformerFactory := informers.NewSharedInformerFactory(client, 30*time.Second)
	informer := kubeInformerFactory.Core().V1().Pods()

	for _, pod := range pods {
		// TODO why is DeepCopy() required here? Without it, each Add() duplicates
		// the first member added.
		err := informer.Informer().GetStore().Add(pod.DeepCopy())
		if err != nil {
			// Should be a programming error
			panic(err)
		}
	}

	return informer.Lister()
}
