package prometheus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultAndValidate(t *testing.T) {
	c := metricConfiguration{}
	err := c.defaultAndValidate(nil)
	assert.NoError(t, err, "nil config provided is ok")
	assert.Equal(t, defaultAggregation, c.Aggregation, "aggregation defaulted")
	assert.Equal(t, defaultRange, c.Range, "range defaulted")

	c = metricConfiguration{}
	err = c.defaultAndValidate(map[string]string{})
	assert.NoError(t, err, "empty config provided is ok")
	assert.Equal(t, defaultAggregation, c.Aggregation, "aggregation defaulted")
	assert.Equal(t, defaultRange, c.Range, "range defaulted")

	c = metricConfiguration{}
	err = c.defaultAndValidate(map[string]string{
		"aggregation": "max",
		"range":       "5m",
	})
	assert.NoError(t, err, "good config")
	assert.Equal(t, "max", c.Aggregation, "aggregation not defaulted if provided")
	assert.Equal(t, "5m", c.Range, "range not defaulted if provided")
	assert.Equal(t, defaultNodeCPUMetricName, c.NodeCPUMetricName, "cpu metric name defaulted if not provided")

	c = metricConfiguration{}
	err = c.defaultAndValidate(map[string]string{
		"cpuMetricName": "node_cpu",
	})
	assert.NoError(t, err, "good config")
	assert.Equal(t, "node_cpu", c.NodeCPUMetricName, "cpu metric name not defaulted if provided")

	c = metricConfiguration{}
	err = c.defaultAndValidate(map[string]string{
		"aggregation": "not-valid",
		"range":       "5m",
	})
	assert.Error(t, err, "bad aggregation")

	c = metricConfiguration{}
	err = c.defaultAndValidate(map[string]string{
		"aggregation": "max",
		"range":       "asdf",
	})
	assert.Error(t, err, "bad range")

	c = metricConfiguration{}
	err = c.defaultAndValidate(map[string]string{
		"aggregation":   "max",
		"range":         "1m",
		"cpuMetricName": "bad",
	})
	assert.Error(t, err, "bad CPU metric name")

	c = metricConfiguration{}
	err = c.defaultAndValidate(map[string]string{
		"aggregation":   "max",
		"range":         "1m",
		"cpuMetricName": "bad",
	})
	assert.Error(t, err, "bad CPU metric name")
}
