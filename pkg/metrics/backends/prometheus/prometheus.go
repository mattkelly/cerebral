package prometheus

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
	"time"

	"github.com/pkg/errors"

	prometheusclient "github.com/prometheus/client_golang/api"
	prometheus "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/containership/cerebral/pkg/metrics"
	"github.com/containership/cluster-manager/pkg/log"
)

// Backend implements a metrics backend for Prometheus. It requires a pod
// lister so that it can gather info about the prom-exporter pods. Pods
// accessed via the lister must not be mutated.
type Backend struct {
	prometheus prometheus.API

	podLister corelistersv1.PodLister
}

// Average CPU usage across the given nodes for the given range
const cpuQueryTemplateString = `
100 - (
	{{.Aggregation}}(
		irate({{.NodeCPUMetricName}}{mode='idle',instance=~'{{.InstancesRegex}}'}[{{.Range}}])
	) * 100
)`

var cpuQueryTemplate = template.Must(template.New("cpu").Parse(cpuQueryTemplateString))

// Average memory usage across the given nodes for the given range
const memoryQueryTemplateString = `
100 * {{.Aggregation}}(
	1 - ((
			avg_over_time(node_memory_MemFree{instance=~'{{.InstancesRegex}}'}[{{.Range}}])
			  + avg_over_time(node_memory_Cached{instance=~'{{.InstancesRegex}}'}[{{.Range}}])
			  + avg_over_time(node_memory_Buffers{instance=~'{{.InstancesRegex}}'}[{{.Range}}])
		  )
		  / avg_over_time(node_memory_MemTotal{instance=~'{{.InstancesRegex}}'}[{{.Range}}]))
)`

var memoryQueryTemplate = template.Must(template.New("mem").Parse(memoryQueryTemplateString))

// NewClient returns a new client for talking to a Prometheus Backend, or an error
func NewClient(address string, podLister corelistersv1.PodLister) (metrics.Backend, error) {
	if address == "" {
		// Under the hood, prometheusclient uses url.Parse() which allows
		// relative URLs, etc. Empty would be allowed, so disallow it
		// explicitly here.
		return nil, errors.New("address must not be empty")
	}

	if podLister == nil {
		return nil, errors.New("pod lister must be provided")
	}

	client, err := prometheusclient.NewClient(prometheusclient.Config{
		Address: address,
	})
	if err != nil {
		return nil, errors.Wrap(err, "instantiating prometheus client")
	}

	api := prometheus.NewAPI(client)

	return Backend{
		prometheus: api,
		podLister:  podLister,
	}, nil
}

// GetValue implements the metrics.Backend interface
func (b Backend) GetValue(metric string, configuration map[string]string, nodes []corev1.Node) (float64, error) {
	podIPs, err := b.getNodeExporterPodIPsOnNodes(nodes)
	if err != nil {
		return 0, errors.Wrapf(err, "getting Prometheus node exporter pod IPs for metric %s", metric)
	}

	switch metric {
	case MetricCPU.String():
		query, _ := buildCPUQuery(podIPs, configuration)
		return b.performQuery(query)

	case MetricMemory.String():
		query, _ := buildMemoryQuery(podIPs, configuration)
		return b.performQuery(query)

	default:
		return 0, errors.Errorf("unknown metric %q", metric)
	}
}

func (b Backend) getNodeExporterPodIPsOnNodes(nodes []corev1.Node) ([]string, error) {
	var podIPs []string

	// TODO we should rethink how we do all of the below, potentially using b.Targets()
	// to help. See https://github.com/containership/cerebral/issues/15.
	selector := labels.NewSelector()
	l, _ := labels.NewRequirement("prom-exporter", selection.Equals, []string{"node"})
	selector = selector.Add(*l)

	// TODO if we could use a FieldSelector including Pod.Spec.NodeName then we
	// could avoid the gross O(n^2) below. Is there a way to do that without
	// having to query the API directly?
	pods, err := b.podLister.List(selector)
	if err != nil {
		return nil, errors.Wrap(err, "listing node exporter pods")
	}
	// Only filter in prom-exporter pods that belong to a node we care about
	for _, pod := range pods {
		for _, node := range nodes {
			if pod.Spec.NodeName == node.ObjectMeta.Name {
				podIPs = append(podIPs, pod.Status.PodIP)
			}
		}
	}

	if len(podIPs) != len(nodes) {
		return nil, errors.Errorf("found %d node exporter pods for %d nodes", len(podIPs), len(nodes))
	}

	return podIPs, nil
}

func (b Backend) performQuery(query string) (float64, error) {
	log.Debugf("Performing prometheus query: %s", query)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	val, err := b.prometheus.Query(ctx, query, time.Time{})
	if err != nil {
		return 0, errors.Wrapf(err, "querying prometheus with string %q", query)
	}

	var result float64
	switch v := val.(type) {
	case model.Vector:
		if len(v) != 1 {
			return 0, errors.Errorf("expected vector to have a single element but it has %d", len(v))
		}

		result = float64(v[0].Value)

	default:
		return 0, errors.Errorf("unexpected prometheus value type %T: %#v", v, v)
	}

	return result, nil
}

func buildCPUQuery(instanceIPs []string, configuration map[string]string) (string, error) {
	config := cpuMetricConfiguration{}
	if err := config.defaultAndValidate(configuration); err != nil {
		return "", errors.Wrap(err, "validating configuration")
	}

	config.InstancesRegex = buildInstancesRegex(instanceIPs)

	var out bytes.Buffer
	if err := cpuQueryTemplate.Execute(&out, config); err != nil {
		return "", err
	}

	return out.String(), nil
}

func buildMemoryQuery(instanceIPs []string, configuration map[string]string) (string, error) {
	config := metricConfiguration{}
	if err := config.defaultAndValidate(configuration); err != nil {
		return "", errors.Wrap(err, "validating configuration")
	}

	config.InstancesRegex = buildInstancesRegex(instanceIPs)

	var out bytes.Buffer
	if err := memoryQueryTemplate.Execute(&out, config); err != nil {
		return "", err
	}

	return out.String(), nil
}

func buildInstancesRegex(instanceIPs []string) string {
	var regex string
	for i, ip := range instanceIPs {
		regex += fmt.Sprintf("%s:.*", ip)
		if i != len(instanceIPs)-1 {
			regex += "|"
		}
	}

	return regex
}
