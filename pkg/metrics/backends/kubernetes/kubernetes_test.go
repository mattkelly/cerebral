package kubernetes

import (
	"math"
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

	podSucceeded = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-succeeded-0",
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
		Status: corev1.PodStatus{
			Phase: corev1.PodPhase("Succeeded"),
		},
	}

	podRunning = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-running-1",
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
							corev1.ResourceEphemeralStorage:    *resource.NewQuantity(1024, resource.DecimalSI),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPhase("Running"),
		},
	}

	podRunning2 = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-running-2",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: node2.ObjectMeta.Name,
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:                    resource.MustParse("100m"),
							corev1.ResourceName("nvidia.com/gpu"): resource.MustParse("3"),
							corev1.ResourceMemory:                 *resource.NewQuantity(1024, resource.DecimalSI),
							corev1.ResourceEphemeralStorage:       *resource.NewQuantity(2048, resource.DecimalSI),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPhase("Running"),
		},
	}

	podFailed = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-failed-2",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: node1.ObjectMeta.Name,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPhase("Failed"),
		},
	}
)

var backend = Backend{
	nodeLister: kubernetestest.BuildNodeLister([]corev1.Node{*node0, *node1, *node2}),
	podLister:  buildPodLister([]corev1.Pod{*podFailed, *podRunning, *podRunning2, *podFailed}),
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
	var emptyRunningPodNodeList = []*corev1.Node{node0}
	var singleRunningPodNodeList = []*corev1.Node{node0, node1}
	var multipleRunningPodNodeList = []*corev1.Node{node0, node1, node2}

	pods, _ := backend.getAllocatedPodsOnNodes(nil)
	assert.Empty(t, pods, "nil node list returns empty array")

	pods, _ = backend.getAllocatedPodsOnNodes(emptyNodeList)
	assert.Empty(t, pods, "empty node list returns empty array")

	pods, _ = backend.getAllocatedPodsOnNodes(emptyRunningPodNodeList)
	assert.Empty(t, pods, "node list with only pods in failed or succeeded phase returns empty array")

	pods, _ = backend.getAllocatedPodsOnNodes(singleRunningPodNodeList)
	assert.Equal(t, 1, len(pods), "returns single result")

	pods, _ = backend.getAllocatedPodsOnNodes(multipleRunningPodNodeList)
	assert.Equal(t, 2, len(pods), "returns two results")
}

func TestCalculateCPUAllocationPercentage(t *testing.T) {
	var nodeList = []*corev1.Node{node0, node1, node2}
	var podList, _ = backend.getAllocatedPodsOnNodes(nodeList)

	percentage := backend.calculateCPUAllocationPercentage(podList, nodeList)
	assert.Equal(t, float64(10), percentage, "returns correct allocation percentage")
}

func TestCalculateGPUAllocationPercentage(t *testing.T) {
	// 6 total gpus, 3 amd requested
	var nodeList = []*corev1.Node{node0, node1}
	var podList, _ = backend.getAllocatedPodsOnNodes(nodeList)
	percentage := backend.calculateGPUAllocationPercentage(podList, nodeList)
	assert.Equal(t, float64(75), percentage, "returns correct allocation percentage for 1 pod requesting 3 nvidia gpus")

	// 6 total gpus, 3 nvidia requested
	nodeList = []*corev1.Node{node0, node2}
	podList, _ = backend.getAllocatedPodsOnNodes(nodeList)
	percentage = backend.calculateGPUAllocationPercentage(podList, nodeList)
	assert.Equal(t, float64(75), percentage, "returns correct allocation percentage for 1 pod requesting 3 amd gpus")

	// 6 total gpus, 3 amd and 3 nvidia requested, failed and succeeded are not included calculation
	nodeList = []*corev1.Node{node0, node1, node2}
	podList, _ = backend.getAllocatedPodsOnNodes(nodeList)
	percentage = backend.calculateGPUAllocationPercentage(podList, nodeList)
	assert.Equal(t, float64(100), percentage, "returns correct allocation percentage for 2 pods request 3 amd and 3 nvidia gpus")
}

func TestCalculateMemoryAllocationPercentage(t *testing.T) {
	var nodeList = []*corev1.Node{node0, node1, node2}
	var podList, _ = backend.getAllocatedPodsOnNodes(nodeList)

	percentage := backend.calculateMemoryAllocationPercentage(podList, nodeList)
	assert.Equal(t, float64(50), percentage, "returns correct allocation percentage")
}

func TestCalculateEphemeralStorageAllocationPercentage(t *testing.T) {
	var nodeList = []*corev1.Node{node0, node1, node2}
	var podList, _ = backend.getAllocatedPodsOnNodes(nodeList)

	percentage := backend.calculateEphemeralStorageAllocationPercentage(podList, nodeList)
	assert.Equal(t, float64(25), percentage, "returns correct allocation percentage")
}

func TestCalculatePodAllocationPercentage(t *testing.T) {
	var nodeList = []*corev1.Node{node0, node1, node2}
	var podList, _ = backend.getAllocatedPodsOnNodes(nodeList)

	percentage := backend.calculatePodAllocationPercentage(podList, nodeList)

	// we need to round to do a sane assertion
	assert.Equal(t, float64(33.33), math.Floor(percentage*100)/100, "returns correct allocation percentage")
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
