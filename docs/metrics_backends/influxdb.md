# InfluxDB Metrics Backend

## Description
The InfluxDB metrics backend interfaces with InfluxDB to expose CPU, memory, and custom metrics gathered by querying the InfluxDB API.

## Configuration
| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `address` | true | | InfluxDB API address used to query metrics. Should be in the format: `scheme://host:<port>` |

## Example
```yaml
apiVersion: cerebral.containership.io/v1alpha1
kind: MetricsBackend
metadata:
  name: influxdb
spec:
  type: influxdb
  configuration:
    address: http://influxdb.containership-core.svc.cluster.local:8086
```

## Available Metrics
* [CPU Percent Utilization](#cpu-percent-utilization)
* [Memory Percent Utilization](#memory-percent-utilization)
* [Custom](#custom)

### CPU Percent Utilization

#### Description
Returns the percent of utilized CPUs across the nodes in the autoscaling group.

#### Metric
`cpu_percent_utilization`

#### Configuration
| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `aggregation` | false | `mean` | InfluxDB aggregation function used in the query. See [the official documentation](https://docs.influxdata.com/influxdb/v1.7/query_language/functions/#aggregations) for more details. |
| `database` | false | `telegraf` | InfluxDB database name used in the query. |
| `range` | false | `1m` | The time range over which to perform the InfluxDB query. |
| `retentionPolicy` | false | `rp_90d` | InfluxDB retention policy used in the query. See [the official documentation](https://docs.influxdata.com/influxdb/v1.7/query_language/database_management/#retention-policy-management) for more details. |

#### Example
```yaml
apiVersion: cerebral.containership.io/v1alpha1
kind: AutoscalingPolicy
metadata:
  name: cpu-example-policy
spec:
  metric: cpu_percent_utilization
  metricConfiguration:
    database: non-default-example
  metricsBackend: influxdb
  pollInterval: 15
  samplePeriod: 300
  scalingPolicy:
    scaleDown:
      adjustmentType: absolute
      adjustmentValue: 1
      comparisonOperator: <=
      threshold: 30
    scaleUp:
      adjustmentType: absolute
      adjustmentValue: 2
      comparisonOperator: '>='
      threshold: 70
```

### Memory Percent Utilization

#### Description
Returns the percent of utilized memory across the nodes in the autoscaling group.

#### Metric
`memory_percent_utilization`

#### Configuration
| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `aggregation` | false | `mean` | InfluxDB aggregation function used in the query. See [the official documentation](https://docs.influxdata.com/influxdb/v1.7/query_language/functions/#aggregations) for more details. |
| `database` | false | `telegraf` | InfluxDB database name used in the query. |
| `range` | false | `1m` | The time range over which to perform the InfluxDB query. |
| `retentionPolicy` | false | `rp_90d` | InfluxDB retention policy used in the query. See [the official documentation](https://docs.influxdata.com/influxdb/v1.7/query_language/database_management/#retention-policy-management) for more details. |

#### Example
```yaml
apiVersion: cerebral.containership.io/v1alpha1
kind: AutoscalingPolicy
metadata:
  name: memory-example-policy
spec:
  metric: memory_percent_utilization
  metricConfiguration:
    database: non-default-example
  metricsBackend: influxdb
  pollInterval: 15
  samplePeriod: 300
  scalingPolicy:
    scaleDown:
      adjustmentType: absolute
      adjustmentValue: 1
      comparisonOperator: <=
      threshold: 30
    scaleUp:
      adjustmentType: absolute
      adjustmentValue: 2
      comparisonOperator: '>='
      threshold: 70
```

### Custom

#### Description
Returns the result of the custom query across the nodes in the autoscaling group.

#### Metric
`custom`

#### Configuration
| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `query` | true | | Query that will be executed against the InfluxDB API. See [the official documentation](https://docs.influxdata.com/influxdb/v1.7/query_language/spec/) for more details. |

**Note:** In order for the query to target the hosts that are part of the AutoscalingGroup, the `query` configuration parameter should contain a templated `WHERE` clause such as:
```
WHERE time > now() - 1m AND {{.HostList}}
```

#### Example
The below custom metric example recreates the built-in `cpu_percent_utilization` metric by leveraging the same query:
```yaml
apiVersion: cerebral.containership.io/v1alpha1
kind: AutoscalingPolicy
metadata:
  name: custom-example-policy
spec:
  metric: custom
  metricConfiguration:
    query: SELECT mean("usage_idle") AS "mean_usage_idle" FROM "telegraf"."rp_90d"."cpu" WHERE time > now() - 1m AND {{.HostList}}
  metricsBackend: influxdb
  pollInterval: 15
  samplePeriod: 300
  scalingPolicy:
    scaleDown:
      adjustmentType: absolute
      adjustmentValue: 1
      comparisonOperator: <=
      threshold: 30
    scaleUp:
      adjustmentType: absolute
      adjustmentValue: 2
      comparisonOperator: '>='
      threshold: 70
```
