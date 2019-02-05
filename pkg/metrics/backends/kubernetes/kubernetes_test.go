package kubernetes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/containership/cerebral/pkg/kubernetestest"
)

var (
	nodeAllocatable = corev1.ResourceList{
		corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("1"),
		corev1.ResourceName("amd.com/gpu"):    resource.MustParse("1"),
		corev1.ResourceCPU:                    resource.MustParse("1"),
		corev1.ResourceMemory:                 *resource.NewQuantity(1024, resource.DecimalSI),
		corev1.ResourceEphemeralStorage:       *resource.NewQuantity(4096, resource.DecimalSI),
		corev1.ResourcePods:                   *resource.NewQuantity(2, resource.DecimalSI),
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
							corev1.ResourceCPU:                    resource.MustParse("100m"),
							corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("3"),
							corev1.ResourceMemory:                 *resource.NewQuantity(256, resource.DecimalSI),
							corev1.ResourceEphemeralStorage:       *resource.NewQuantity(1024, resource.DecimalSI),
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
							corev1.ResourceCPU:                 resource.MustParse("200m"),
							corev1.ResourceName("amd.com/gpu"): resource.MustParse("3"),
							corev1.ResourceMemory:              *resource.NewQuantity(512, resource.DecimalSI),
							corev1.ResourceEphemeralStorage:    *resource.NewQuantity(2048, resource.DecimalSI),
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
	nodeLister: kubernetestest.BuildNodeLister([]corev1.Node{*node0, *node1, *node2}),
	podLister:  buildPodLister([]corev1.Pod{*pod0, *pod1, *pod2}),
}

func TestGetValue(t *testing.T) {
	_, err := backend.GetValue("cpu_percent_allocation", nil, nil)
	assert.NoError(t, err, "successfully get cpu allocation metric")

	_, err = backend.GetValue("gpu_percent_allocation", nil, nil)
	assert.NoError(t, err, "successfully get gpu allocation metric")

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
	assert.Equal(t, float64(10), percentage, "returns correct allocation percentage")
}

func TestCalculateGPUAllocationPercentage(t *testing.T) {
	// 6 total gpus, 3 nvidia requested
	var podList = []*corev1.Pod{pod0}
	var nodeList = []*corev1.Node{node0, node1, node2}
	percentage := backend.calculateGPUAllocationPercentage(podList, nodeList)
	assert.Equal(t, float64(50), percentage, "returns correct allocation percentage for 1 pod requesting 3 nvidia gpus")

	// 6 total gpus, 3 amd requested
	podList = []*corev1.Pod{pod1}
	percentage = backend.calculateGPUAllocationPercentage(podList, nodeList)
	assert.Equal(t, float64(50), percentage, "returns correct allocation percentage for 1 pod requesting 3 amd gpus")

	// 6 total gpus, 3 amd and 3 nvidia requested
	podList = []*corev1.Pod{pod0, pod1}
	percentage = backend.calculateGPUAllocationPercentage(podList, nodeList)
	assert.Equal(t, float64(100), percentage, "returns correct allocation percentage for 2 pods request 3 amd and 3 nvidia gpus")
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
