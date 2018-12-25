package influxdb

import (
	"encoding/json"
	"regexp"

	"github.com/pkg/errors"
)

// Metric is a metric exposed by this backend
type Metric int

const (
	// MetricCPUPercentUtilization is used to gather info about the CPU usage of nodes
	MetricCPUPercentUtilization Metric = iota
	// MetricMemoryPercentUtilization is used to gather info about the Memory usage of nodes
	MetricMemoryPercentUtilization
	// MetricCustom is used to perform a custom InfluxDB query
	MetricCustom
)

// String is a stringer for Metric
func (m Metric) String() string {
	switch m {
	case MetricCPUPercentUtilization:
		return "cpu_percent_utilization"
	case MetricMemoryPercentUtilization:
		return "memory_percent_utilization"
	case MetricCustom:
		return "custom"
	}

	return "unknown"
}

var validAggregations = []string{
	"count",    // number of non-null field values
	"distinct", // list of unique field values
	"integral", // area under the curve for subsequent field values
	"mean",     // arithmetic mean (average) of field values
	"median",   // middle value from a sorted list of field values
	"mode",     // most frequent value in a list of field values
	"spread",   // difference between the minimum and maximum field values
	"stdev",    // standard deviation of field values
	"sum",      // sum of field values
	"max",      // greatest field value
	"min",      // lowest field value
}

// See https://docs.influxdata.com/influxdb/v1.7/query_language/spec/#durations
var validRangeRegex = regexp.MustCompile(`^\d+[smhdwy]$`)

const defaultAggregation = "mean"
const defaultDatabase = "telegraf"
const defaultRange = "1m"
const defaultRetentionPolicy = "rp_90d"

// TODO consider splitting into multiple types instead of overloading this
// single struct and ignoring irrelevant fields
type metricConfiguration struct {
	// -- Generic
	Aggregation     string `json:"aggregation"`
	Database        string `json:"database"`
	Range           string `json:"range"`
	RetentionPolicy string `json:"retentionPolicy"`

	// -- Not user-specifiable
	// Unfortunately we're required to export these fields for use in templates
	HostList string
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

	if err := c.defaultAndValidateDB(); err != nil {
		return err
	}

	if err := c.defaultAndValidateRange(); err != nil {
		return err
	}

	if err := c.defaultAndValidateRetentionPolicy(); err != nil {
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

func (c *metricConfiguration) defaultAndValidateDB() error {
	if c.Database == "" {
		c.Database = defaultDatabase
	}

	return nil
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

func (c *metricConfiguration) defaultAndValidateRetentionPolicy() error {
	if c.RetentionPolicy == "" {
		c.RetentionPolicy = defaultRetentionPolicy
	}

	return nil
}
