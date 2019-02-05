package prometheus

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/prometheus/common/model"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/containership/cerebral/pkg/kubernetestest"
	"github.com/containership/cerebral/pkg/metrics/backends/prometheus/mocks"
)

var (
	validURL = "http://localhost:9000"

	duration = "5m"

	goodConfiguration = map[string]string{
		"aggregation": "avg",
	}

	goodCustomQueryConfiguration = map[string]string{
		"query": `
			100 - (
				{{.Aggregation}}(
					irate({{.NodeCPUMetricName}}{mode='idle',instance=~'{{.PodIPsRegex}}'}[{{.Range}}])
				) * 100
			)
		`,
	}

	badAggregationConfiguration = map[string]string{
		"aggregation": "invalid-aggregation",
	}

	emptyConfiguration = map[string]string{}

	noIPs       []string
	oneIP       = []string{"10.0.0.1"}
	multipleIPs = []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}

	podIP0 = "192.168.0.1"
	podIP1 = "192.168.1.1"
)

var (
	promNode0 = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prom-0",
		},
	}
	promNode1 = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prom-1",
		},
	}
	otherNode0 = corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "other-0",
		},
	}

	promPodOnNode0 = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prom-exporter-node-asdfg",
			Namespace: "kube-system",
			Labels: map[string]string{
				"prom-exporter": "node",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: promNode0.ObjectMeta.Name,
		},
		Status: corev1.PodStatus{
			PodIP: podIP0,
		},
	}

	promPodOnNode1 = corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "prom-exporter-node-hijkl",
			Namespace: "containership-core",
			Labels: map[string]string{
				"prom-exporter": "node",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: promNode1.ObjectMeta.Name,
		},
		Status: corev1.PodStatus{
			PodIP: podIP1,
		},
	}
)

func TestNewClient(t *testing.T) {
	// Should never fail with any valid URL because it's only constructing an
	// http.Client under the hood
	client, err := NewClient(validURL, corelistersv1.NewNodeLister(nil), corelistersv1.NewPodLister(nil))
	assert.NotNil(t, client)
	assert.NoError(t, err, "any valid URL is ok")

	client, err = NewClient("", corelistersv1.NewNodeLister(nil), corelistersv1.NewPodLister(nil))
	assert.Error(t, err, "error on empty URL")

	_, err = NewClient(validURL, nil, corelistersv1.NewPodLister(nil))
	assert.Error(t, err, "error on nil NodeLister")

	_, err = NewClient(validURL, corelistersv1.NewNodeLister(nil), nil)
	assert.Error(t, err, "error on nil PodLister")
}

func TestGetValue(t *testing.T) {
	nodeLister := kubernetestest.BuildNodeLister(nil)
	podLister := buildPodLister(nil)

	mockProm := mocks.API{}
	// Return error
	mockProm.On("Query", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("some prometheus error")).Once()

	backend := Backend{
		prometheus: &mockProm,
		nodeLister: nodeLister,
		podLister:  podLister,
	}

	_, err := backend.GetValue("cpu_percent_utilization", goodConfiguration, nil)
	assert.Error(t, err, "error when prometheus errors")

	// Return unexpected nil
	mockProm.On("Query", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil).Once()

	_, err = backend.GetValue("cpu_percent_utilization", goodConfiguration, nil)
	assert.Error(t, err, "error on nil result")

	// Return unexpected non-Vector type
	mockProm.On("Query", mock.Anything, mock.Anything, mock.Anything).
		Return(&model.Scalar{}, nil).Once()

	_, err = backend.GetValue("cpu_percent_utilization", goodConfiguration, nil)
	assert.Error(t, err, "error on non-vector result")

	// Return single element vector as expected
	mockProm.On("Query", mock.Anything, mock.Anything, mock.Anything).
		Return(model.Vector{
			{
				Metric:    model.Metric{},
				Value:     0.5,
				Timestamp: 1234,
			},
		}, nil).Once()

	_, err = backend.GetValue("cpu_percent_utilization", goodConfiguration, nil)
	assert.NoError(t, err, "single element vector is ok")

	// Return single element vector as expected
	mockProm.On("Query", mock.Anything, mock.Anything, mock.Anything).
		Return(model.Vector{
			{
				Metric:    model.Metric{},
				Value:     0.5,
				Timestamp: 1234,
			},
			{
				Metric:    model.Metric{},
				Value:     1.75,
				Timestamp: 1234,
			},
		}, nil).Once()

	_, err = backend.GetValue("cpu_percent_utilization", goodConfiguration, nil)
	assert.Error(t, err, "multiple element vector errors")

	_, err = backend.GetValue("not a valid metric", goodConfiguration, nil)
	assert.Error(t, err, "unknown metric requested")
}

func TestGetNodeExporterPodIPsOnNodes(t *testing.T) {
	emptyPodLister := buildPodLister(nil)

	backend := Backend{
		prometheus: &mocks.API{},
		podLister:  emptyPodLister,
	}

	// Empty cache but no nodes requested
	ips, err := backend.getNodeExporterPodIPsOnNodes(nil)
	assert.NoError(t, err)
	assert.Empty(t, ips, "no nodes with empty pod cache --> no IPs")

	// Fill the cache with valid pods, query using valid nodes
	fullPodLister := buildPodLister([]corev1.Pod{
		promPodOnNode0,
		promPodOnNode1,
	})

	backend.podLister = fullPodLister

	nodes := []*corev1.Node{&promNode0, &promNode1}

	ips, err = backend.getNodeExporterPodIPsOnNodes(nodes)

	assert.NoError(t, err)
	assert.Len(t, ips, len(nodes), "proper number of pod IPs found")
	assert.Contains(t, ips, podIP0, "found expected pod IP 0")
	assert.Contains(t, ips, podIP1, "found expected pod IP 1")

	// Cache still full with valid pods, querying for zero nodes
	ips, err = backend.getNodeExporterPodIPsOnNodes(nil)
	assert.NoError(t, err)
	assert.Empty(t, ips, "no nodes with full pod cache --> no IPs")

	// Add another node which is not running exporter
	nodes = []*corev1.Node{&promNode0, &promNode1, &otherNode0}
	ips, err = backend.getNodeExporterPodIPsOnNodes(nodes)
	assert.Error(t, err, "not every node running node exporter")
}

func TestBuildCPUQuery(t *testing.T) {
	_, err := buildCPUQuery(oneIP, goodConfiguration)
	assert.NoError(t, err, "good configuration is ok")

	_, err = buildCPUQuery(oneIP, emptyConfiguration)
	assert.NoError(t, err, "empty configuration is ok (defaults)")

	_, err = buildCPUQuery(oneIP, badAggregationConfiguration)
	assert.Error(t, err, "invalid aggregation errors")
}

func TestBuildMemoryQuery(t *testing.T) {
	_, err := buildMemoryQuery(oneIP, goodConfiguration)
	assert.NoError(t, err, "good configuration is ok")

	_, err = buildMemoryQuery(oneIP, emptyConfiguration)
	assert.NoError(t, err, "empty configuration is ok (defaults)")

	_, err = buildMemoryQuery(oneIP, badAggregationConfiguration)
	assert.Error(t, err, "invalid aggregation errors")
}

func TestBuildCustomQuery(t *testing.T) {
	_, err := buildCustomQuery(oneIP, goodCustomQueryConfiguration)
	assert.NoError(t, err, "good custom query configuration is ok")

	_, err = buildCustomQuery(oneIP, emptyConfiguration)
	assert.Error(t, err, "empty configuration is invalid (requires query)")
}

func TestBuildPodIPsRegex(t *testing.T) {
	regex := buildPodIPsRegex(nil)
	assert.Empty(t, regex, "nil pod IPs results in empty regex")

	regex = buildPodIPsRegex(noIPs)
	assert.Empty(t, regex, "no pod IPs results in empty regex")

	regex = buildPodIPsRegex(oneIP)
	assert.Equal(t, "10.0.0.1:.*", regex, "single IP regex")

	regex = buildPodIPsRegex(multipleIPs)
	assert.Equal(t, "10.0.0.1:.*|10.0.0.2:.*|10.0.0.3:.*", regex, "multiple IP regex")
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
