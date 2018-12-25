package influxdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/pkg/errors"

	influxdbclient "github.com/influxdata/influxdb/client/v2"

	corelistersv1 "k8s.io/client-go/listers/core/v1"

	"github.com/containership/cerebral/pkg/metrics"
	"github.com/containership/cerebral/pkg/nodeutil"
	"github.com/containership/cluster-manager/pkg/log"
)

// Backend implements a metrics backend for InfluxDB.
type Backend struct {
	influxDB influxdbclient.Client

	nodeLister corelistersv1.NodeLister
}

// Aggregate CPU usage across the given nodes for the given range
const cpuQueryTemplateString = `
SELECT {{.Aggregation}}("usage_idle") AS "mean_usage_idle" FROM "{{.Database}}"."{{.RetentionPolicy}}"."cpu"
WHERE time > now() - {{.Range}} AND {{.HostList}}
`

var cpuQueryTemplate = template.Must(template.New("cpu").Parse(cpuQueryTemplateString))

// Aggregate memory usage across the given nodes for the given range
const memoryQueryTemplateString = `
SELECT {{.Aggregation}}("used_percent") AS "mean_used_percent" FROM "{{.Database}}"."{{.RetentionPolicy}}"."mem"
WHERE time > now() - {{.Range}} AND {{.HostList}}
`

var memoryQueryTemplate = template.Must(template.New("mem").Parse(memoryQueryTemplateString))

// NewClient returns a new client for talking to an InfluxDB Backend, or an error
func NewClient(address string, nodeLister corelistersv1.NodeLister) (metrics.Backend, error) {
	if address == "" {
		// As explicitly stated in the InfluxDB client,
		// Addr should be of the form "http://host:<port>"
		// or "http://[ipv6-host%zone]:<port>".
		return nil, errors.New("address must not be empty")
	}

	if nodeLister == nil {
		return nil, errors.New("node lister must be provided")
	}

	client, err := influxdbclient.NewHTTPClient(influxdbclient.HTTPConfig{
		Addr: address,
	})

	if err != nil {
		return nil, errors.Wrap(err, "instantiating InfluxDB client")
	}

	return Backend{
		influxDB:   client,
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

	hostnames := make([]string, len(nodes))
	for i, node := range nodes {
		hostnames[i] = node.ObjectMeta.Labels["kubernetes.io/hostname"]
	}

	// default and validate the configuration before using it to build
	// and perform the query to InfluxDB
	config := metricConfiguration{}
	if err := config.defaultAndValidate(configuration); err != nil {
		return 0, errors.Wrap(err, "validating configuration")
	}

	switch metric {
	case MetricCPUPercentUtilization.String():
		query, err := buildCPUQuery(hostnames, configuration)
		if err != nil {
			return 0, errors.Wrap(err, "building cpu query")
		}
		return b.performQuery(config.Database, query)

	case MetricMemoryPercentUtilization.String():
		query, err := buildMemoryQuery(hostnames, configuration)
		if err != nil {
			return 0, errors.Wrap(err, "building memory query")
		}
		return b.performQuery(config.Database, query)

	case MetricCustom.String():
		query, err := buildCustomQuery(hostnames, configuration)
		if err != nil {
			return 0, errors.Wrap(err, "building custom query")
		}
		return b.performQuery(config.Database, query)

	default:
		return 0, errors.Errorf("unknown metric %q", metric)
	}
}

func (b Backend) performQuery(db string, query string) (float64, error) {
	log.Debugf("Performing InfluxDB query: %s", query)

	res, err := b.influxDB.Query(influxdbclient.Query{
		Command:  query,
		Database: db,
	})

	switch {
	case err != nil:
		return 0, errors.Wrapf(err, "querying InfluxDB with string %q", query)

	case res == nil:
		return 0, errors.Errorf("querying InfluxDB with string %q returned nil", query)

	case res.Error() != nil:
		return 0, errors.Wrapf(res.Error(), "querying InfluxDB with string %q", query)

	case len(res.Results) != 1:
		return 0, errors.New("querying InfluxDB returned an unexpected number of results")
	}

	var result float64
	v := res.Results[0].Series
	if len(v) != 1 {
		return 0, errors.New("expected Series to have an item")
	}

	if len(v[0].Values) != 1 {
		return 0, errors.Errorf("expected Series Values to have a single value element but it has %d", len(v[0].Values))
	}

	result, err = v[0].Values[0][1].(json.Number).Float64()
	if err != nil {
		return 0, err
	}

	return result, nil
}

func buildCPUQuery(hostnames []string, configuration map[string]string) (string, error) {
	config := metricConfiguration{}
	if err := config.defaultAndValidate(configuration); err != nil {
		return "", errors.Wrap(err, "validating configuration")
	}

	config.HostList = buildHostList(hostnames)

	var out bytes.Buffer
	if err := cpuQueryTemplate.Execute(&out, config); err != nil {
		return "", err
	}

	return out.String(), nil
}

func buildMemoryQuery(hostnames []string, configuration map[string]string) (string, error) {
	config := metricConfiguration{}
	if err := config.defaultAndValidate(configuration); err != nil {
		return "", errors.Wrap(err, "validating configuration")
	}

	config.HostList = buildHostList(hostnames)

	var out bytes.Buffer
	if err := memoryQueryTemplate.Execute(&out, config); err != nil {
		return "", err
	}

	return out.String(), nil
}

// For a custom query, there should be a single `query` key provided in the
// configuration map. No further configuration keys are currently supported.
func buildCustomQuery(hostnames []string, configuration map[string]string) (string, error) {
	var query string
	var ok bool
	query, ok = configuration["query"]
	if !ok {
		return "", errors.New("single configuration key \"query\" must be provided for a custom query")
	}

	config := metricConfiguration{}
	config.HostList = buildHostList(hostnames)

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

func buildHostList(hostnames []string) string {
	// if hostnames is nil or of length zero, simply return "(true)"
	// to match all nodes
	if hostnames == nil || len(hostnames) == 0 {
		return "(true)"
	}

	var hostList string
	for i, hostname := range hostnames {
		hostList += fmt.Sprintf("\"host\"='%s'", hostname)
		if i != len(hostnames)-1 {
			hostList += " OR "
		}
	}

	return fmt.Sprintf("(%s)", hostList)
}
