package prometheus

import (
	"encoding/json"
	"regexp"

	"github.com/pkg/errors"
)

// Metric is a metric exposed by this backend
type Metric int

const (
	// MetricCPU is used to gather info about the CPU usage of nodes
	MetricCPU Metric = iota
	// MetricMemory is used to gather info about the CPU usage of nodes
	MetricMemory
)

// String is a stringer for Metric
func (m Metric) String() string {
	switch m {
	case MetricCPU:
		return "cpu"
	case MetricMemory:
		return "memory"
	}

	return "unknown"
}

var validAggregations = []string{
	"sum",          // calculate sum over dimensions
	"min",          // select minimum over dimensions
	"max",          // select maximum over dimensions
	"avg",          // calculate the average over dimensions
	"stddev",       // calculate population standard deviation over dimensions
	"stdvar",       // calculate population standard variance over dimensions
	"count",        // count number of elements in the vector
	"count_values", // count number of elements with the same value
	"bottomk",      // smallest k elements by sample value
	"topk",         // largest k elements by sample value
	"quantile",     // calculate φ-quantile (0 ≤ φ ≤ 1) over dimensions
}

// See https://prometheus.io/docs/prometheus/latest/querying/basics/#range-vector-selectors
var validRangeRegex = regexp.MustCompile(`^\d+[smhdwy]$`)

const defaultAggregation = "avg"
const defaultRange = "1m"

var validNodeCPUMetricNames = []string{
	"node_cpu_seconds_total", // For Prometheus 0.16.0+
	"node_cpu",               // For Prometheus older than 0.16.0
}

// Default to the older metric name for now, since Containership CKE clusters
// are still launching with an older version for now.
// TODO check the node exporter image version, handle this automatically, and
// remove this config option.
var defaultNodeCPUMetricName = validNodeCPUMetricNames[1]

// TODO consider splitting into multiple types instead of overloading this
// single struct and ignoring irrelevant fields
type metricConfiguration struct {
	// -- Generic
	Aggregation string `json:"aggregation"`
	Range       string `json:"range"`

	// -- CPU
	// NodeCPUMetricName specifies the underlying Prometheus metric name for the cpu metric
	// This is mainly required due to the name changing in Prometheus 0.16.0
	NodeCPUMetricName string `json:"cpuMetricName"`

	// -- Not user-specifiable
	// Unfortunately we're required to export these fields for use in templates
	InstancesRegex string
}

// defaults and validates the metricConfiguration. Intended to be called with an
// empty struct that we'll fill in here using the caller-provided configuration.
func (c *metricConfiguration) defaultAndValidate(configuration map[string]string) error {
	// Round trip the config through JSON parser to populate our struct
	j, _ := json.Marshal(configuration)
	json.Unmarshal(j, c)

	if err := c.defaultAndValidateAggregation(); err != nil {
		return err
	}

	if err := c.defaultAndValidateRange(); err != nil {
		return err
	}

	if err := c.defaultAndValidateNodeCPUMetricName(); err != nil {
		return err
	}

	return nil
}

func (c *metricConfiguration) defaultAndValidateAggregation() error {
	if c.Aggregation == "" {
		c.Aggregation = defaultAggregation
	}

	for _, a := range validAggregations {
		if a == c.Aggregation {
			return nil
		}
	}

	return errors.Errorf("invalid aggregation %s", c.Aggregation)
}

func (c *metricConfiguration) defaultAndValidateRange() error {
	if c.Range == "" {
		c.Range = defaultRange
	}

	if !validRangeRegex.MatchString(c.Range) {
		return errors.Errorf("invalid range %s", c.Range)
	}

	return nil
}

func (c *metricConfiguration) defaultAndValidateNodeCPUMetricName() error {
	if c.NodeCPUMetricName == "" {
		c.NodeCPUMetricName = defaultNodeCPUMetricName
	}

	for _, n := range validNodeCPUMetricNames {
		if n == c.NodeCPUMetricName {
			return nil
		}
	}

	return errors.Errorf("invalid node cpu metric name %s", c.NodeCPUMetricName)
}
