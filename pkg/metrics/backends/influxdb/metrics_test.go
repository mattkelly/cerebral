package influxdb

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
	assert.Equal(t, defaultDatabase, c.Database, "database defaulted")
	assert.Equal(t, defaultRetentionPolicy, c.RetentionPolicy, "retention policy defaulted")

	c = metricConfiguration{}
	err = c.defaultAndValidate(map[string]string{})
	assert.NoError(t, err, "empty config provided is ok")
	assert.Equal(t, defaultAggregation, c.Aggregation, "aggregation defaulted")
	assert.Equal(t, defaultRange, c.Range, "range defaulted")
	assert.Equal(t, defaultDatabase, c.Database, "database defaulted")
	assert.Equal(t, defaultRetentionPolicy, c.RetentionPolicy, "retention policy defaulted")

	c = metricConfiguration{}
	err = c.defaultAndValidate(map[string]string{
		"aggregation":     "max",
		"range":           "5m",
		"database":        "eh",
		"retentionPolicy": "p_100d",
	})
	assert.NoError(t, err, "good config")
	assert.Equal(t, "max", c.Aggregation, "aggregation not defaulted if provided")
	assert.Equal(t, "5m", c.Range, "range not defaulted if provided")
	assert.Equal(t, "eh", c.Database, "database not defaulted if provided")
	assert.Equal(t, "p_100d", c.RetentionPolicy, "retention policy not defaulted if provided")

	c = metricConfiguration{}
	err = c.defaultAndValidate(map[string]string{
		"aggregation":     "BADBADNOTGOOD",
		"range":           "5m",
		"database":        "eh",
		"retentionPolicy": "p_100d",
	})
	assert.Error(t, err, "bad aggregation")

	c = metricConfiguration{}
	err = c.defaultAndValidate(map[string]string{
		"aggregation":     "max",
		"range":           "BADBADNOTGOOD",
		"database":        "eh",
		"retentionPolicy": "p_100d",
	})
	assert.Error(t, err, "bad range")

}
