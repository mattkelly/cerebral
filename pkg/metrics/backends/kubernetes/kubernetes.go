package kubernetes

import (
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/containership/cerebral/pkg/metrics"
	"github.com/containership/cerebral/pkg/nodeutil"
	"github.com/containership/cluster-manager/pkg/log"
)

// Backend implements a metrics backend for Kubernetes. It requires a pod
// lister so that it can gather info about where pods are running.
type Backend struct {
	nodeLister corelistersv1.NodeLister
	podLister  corelistersv1.PodLister
}

// NewClient returns a new client for talking to a Kubernetes Backend, or an error
func NewClient(nodeLister corelistersv1.NodeLister, podLister corelistersv1.PodLister) (metrics.Backend, error) {
	if nodeLister == nil {
		return nil, errors.New("node lister must be provided")
	}

	if podLister == nil {
		return nil, errors.New("pod lister must be provided")
	}

	return Backend{
		nodeLister: nodeLister,
		podLister:  podLister,
	}, nil
}

// GetValue implements the metrics.Backend interface
func (b Backend) GetValue(metric string, configuration map[string]string, nodeSelector map[string]string) (float64, error) {
	selector := nodeutil.GetNodesLabelSelector(nodeSelector)
	nodes, err := b.nodeLister.List(selector)
	if err != nil {
		return 0, errors.Wrap(err, "listing nodes")
	}

	pods, err := b.getPodsOnNodes(nodes)
	if err != nil {
		return 0, errors.Wrapf(err, "getting pods on nodes for metric %s", metric)
	}

	switch metric {
	case MetricCPUPercentAllocation.String():
		value := b.calculateCPUAllocationPercentage(pods, nodes)
		return value, nil

	case MetricMemoryPercentAllocation.String():
		value := b.calculateMemoryAllocationPercentage(pods, nodes)
		return value, nil

	case MetricEphemeralStoragePercentAllocation.String():
		value := b.calculateEphemeralStorageAllocationPercentage(pods, nodes)
		return value, nil

	case MetricPodPercentAllocation.String():
		value := b.calculatePodAllocationPercentage(pods, nodes)
		return value, nil

	default:
		return 0, errors.Errorf("unknown metric %q", metric)
	}
}

func (b Backend) getPodsOnNodes(nodes []*corev1.Node) ([]*corev1.Pod, error) {
	var podsOnNodes []*corev1.Pod

	// Pass an empty selector to list all pods
	pods, err := b.podLister.List(labels.NewSelector())
	if err != nil {
		return nil, errors.Wrap(err, "listing pods")
	}

	// Only filter to pods running on nodes we care about
	for _, pod := range pods {
		for _, node := range nodes {
			if pod.Spec.NodeName == node.ObjectMeta.Name {
				podsOnNodes = append(podsOnNodes, pod)
			}
		}
	}

	return podsOnNodes, nil
}

func (b Backend) calculateCPUAllocationPercentage(pods []*corev1.Pod, nodes []*corev1.Node) float64 {
	log.Debugf("Performing cpu allocation calculation of %d pods across %d nodes", len(pods), len(nodes))

	var allocatableCPUs, requestedCPUs int64

	// calculate sum of allocatable CPUs across nodes
	for _, node := range nodes {
		allocatableCPUs += node.Status.Allocatable.Cpu().Value()
	}

	// calculate sum of requested CPUs across pods
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			requestedCPUs += container.Resources.Requests.Cpu().Value()
		}
	}

	return (100 * (float64(requestedCPUs) / float64(allocatableCPUs)))
}

func (b Backend) calculateMemoryAllocationPercentage(pods []*corev1.Pod, nodes []*corev1.Node) float64 {
	log.Debugf("Performing memory allocation calculation of %d pods across %d nodes", len(pods), len(nodes))

	var allocatableMemory, requestedMemory int64

	// calculate sum of allocatable memory across nodes
	for _, node := range nodes {
		allocatableMemory += node.Status.Allocatable.Memory().Value()
	}

	// calculate sum of requested memory across pods
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			requestedMemory += container.Resources.Requests.Memory().Value()
		}
	}

	return (100 * (float64(requestedMemory) / float64(allocatableMemory)))
}

func (b Backend) calculateEphemeralStorageAllocationPercentage(pods []*corev1.Pod, nodes []*corev1.Node) float64 {
	log.Debugf("Performing ephemeral storage allocation calculation of %d pods across %d nodes", len(pods), len(nodes))

	var allocatableStorage, requestedStorage int64

	// calculate sum of allocatable ephemeral storage across nodes
	for _, node := range nodes {
		allocatableStorage += node.Status.Allocatable.StorageEphemeral().Value()
	}

	// calculate sum of requested ephemeral storage across pods
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			requestedStorage += container.Resources.Requests.StorageEphemeral().Value()
		}
	}

	return (100 * (float64(requestedStorage) / float64(allocatableStorage)))
}

func (b Backend) calculatePodAllocationPercentage(pods []*corev1.Pod, nodes []*corev1.Node) float64 {
	log.Debugf("Performing pod allocation calculation of %d pods across %d nodes", len(pods), len(nodes))

	var allocatablePods int64

	// calculate sum of allocatable pods across nodes
	for _, node := range nodes {
		allocatablePods += node.Status.Allocatable.Pods().Value()
	}

	return (100 * (float64(len(pods)) / float64(allocatablePods)))
}
