package prometheus

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"

	prometheusclient "github.com/prometheus/client_golang/api"
	prometheus "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	corev1 "k8s.io/api/core/v1"
	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/containership/cerebral/pkg/metrics"
	"github.com/containership/cerebral/pkg/nodeutil"
	"github.com/containership/cluster-manager/pkg/log"
)

// Backend implements a metrics backend for Prometheus. It requires a pod
// lister so that it can gather info about the prom-exporter pods. Pods
// accessed via the lister must not be mutated.
type Backend struct {
	prometheus prometheus.API

	nodeLister corelistersv1.NodeLister
}

const (
	prometheusRequestTimeout = 10 * time.Second
)

// Average CPU usage across the given nodes for the given range
const cpuQueryTemplateString = `
100 - (
	{{.Aggregation}}(
		irate({{.NodeCPUMetricName}}{mode='idle',instance=~'{{.PodIPsRegex}}'}[{{.Range}}])
	) * 100
)`

var cpuQueryTemplate = template.Must(template.New("cpu").Parse(cpuQueryTemplateString))

// Average memory usage across the given nodes for the given range
const memoryQueryTemplateString = `
100 * {{.Aggregation}}(
	1 - (avg_over_time(node_memory_MemAvailable{instance=~'{{.PodIPsRegex}}'}[{{.Range}}])
		  / avg_over_time(node_memory_MemTotal{instance=~'{{.PodIPsRegex}}'}[{{.Range}}]))
)`

var memoryQueryTemplate = template.Must(template.New("mem").Parse(memoryQueryTemplateString))

// NewClient returns a new client for talking to a Prometheus Backend, or an error
func NewClient(address string, nodeLister corelistersv1.NodeLister) (metrics.Backend, error) {
	if address == "" {
		// Under the hood, prometheusclient uses url.Parse() which allows
		// relative URLs, etc. Empty would be allowed, so disallow it
		// explicitly here.
		return nil, errors.New("address must not be empty")
	}

	if nodeLister == nil {
		return nil, errors.New("node lister must be provided")
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
		nodeLister: nodeLister,
	}, nil
}

// GetValue implements the metrics.Backend interface
func (b Backend) GetValue(metric string, configuration map[string]string, nodeSelector map[string]string) (float64, error) {
	selector := nodeutil.GetNodesLabelSelector(nodeSelector)
	nodes, err := b.nodeLister.List(selector)
	if err != nil {
		return 0, errors.Wrap(err, "listing nodes")
	}

	podIPs, err := b.getNodeExporterPodIPsOnNodes(nodes)
	if err != nil {
		return 0, errors.Wrapf(err, "getting Prometheus node exporter pod IPs for metric %s", metric)
	}

	switch metric {
	case MetricCPUPercentUtilization.String():
		query, err := buildCPUQuery(podIPs, configuration)
		if err != nil {
			return 0, errors.Wrap(err, "building query")
		}
		return b.performQuery(query)

	case MetricMemoryPercentUtilization.String():
		query, err := buildMemoryQuery(podIPs, configuration)
		if err != nil {
			return 0, errors.Wrap(err, "building query")
		}
		return b.performQuery(query)

	case MetricCustom.String():
		query, err := buildCustomQuery(podIPs, configuration)
		if err != nil {
			return 0, errors.Wrap(err, "building query")
		}
		return b.performQuery(query)

	default:
		return 0, errors.Errorf("unknown metric %q", metric)
	}
}

func (b Backend) getNodeExporterPodIPsOnNodes(nodes []*corev1.Node) ([]string, error) {
	var podIPs []string

	ctx, cancel := context.WithTimeout(context.Background(), prometheusRequestTimeout)
	defer cancel()

	// Filter only prom-exporter job, and further filter down by node IPs
	targets, _ := b.prometheus.Targets(ctx)
	for _, active := range targets.Active {
		jobName := string(active.DiscoveredLabels["job"])
		split := strings.Split(jobName, "/")
		if split[1] == "node-export-monitor" {
			for _, node := range nodes {
				if string(active.DiscoveredLabels["__meta_kubernetes_pod_node_name"]) == node.ObjectMeta.Name {
					podIPs = append(podIPs, string(active.DiscoveredLabels["__meta_kubernetes_pod_ip"]))
				}
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

	ctx, cancel := context.WithTimeout(context.Background(), prometheusRequestTimeout)
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

func buildCPUQuery(podIPs []string, configuration map[string]string) (string, error) {
	config := metricConfiguration{}
	if err := config.defaultAndValidate(configuration); err != nil {
		return "", errors.Wrap(err, "validating configuration")
	}

	config.PodIPsRegex = buildPodIPsRegex(podIPs)

	var out bytes.Buffer
	if err := cpuQueryTemplate.Execute(&out, config); err != nil {
		return "", err
	}

	return out.String(), nil
}

func buildMemoryQuery(podIPs []string, configuration map[string]string) (string, error) {
	config := metricConfiguration{}
	if err := config.defaultAndValidate(configuration); err != nil {
		return "", errors.Wrap(err, "validating configuration")
	}

	config.PodIPsRegex = buildPodIPsRegex(podIPs)

	var out bytes.Buffer
	if err := memoryQueryTemplate.Execute(&out, config); err != nil {
		return "", err
	}

	return out.String(), nil
}

// For a custom query, there should be a single `query` key provided in the
// configuration map. No further configuration keys are currently supported.
func buildCustomQuery(podIPs []string, configuration map[string]string) (string, error) {
	var query string
	var ok bool
	query, ok = configuration["query"]
	if !ok {
		return "", errors.New("single configuration key \"query\" must be provided for a custom query")
	}

	config := metricConfiguration{}
	config.PodIPsRegex = buildPodIPsRegex(podIPs)

	template, err := template.New("query").Parse(query)
	if err != nil {
		return "", errors.Wrap(err, "parsing custom query template")
	}

	var out bytes.Buffer
	if err := template.Execute(&out, config); err != nil {
		return "", err
	}

	return out.String(), nil
}

func buildPodIPsRegex(podIPs []string) string {
	var regex string
	for i, ip := range podIPs {
		regex += fmt.Sprintf("%s:.*", ip)
		if i != len(podIPs)-1 {
			regex += "|"
		}
	}

	return regex
}
