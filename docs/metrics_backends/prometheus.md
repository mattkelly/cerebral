# Prometheus Metrics Backend

## Description
The Prometheus metrics backend interfaces with Prometheus to expose CPU, memory, and custom metrics gathered by querying the Prometheus API.

## Configuration
| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `address` | true | | Prometheus API address used to query metrics. Should be in the format: `scheme://host:<port>` |

## Example
```yaml
apiVersion: cerebral.containership.io/v1alpha1
kind: MetricsBackend
metadata:
  name: prometheus
spec:
  type: prometheus
  configuration:
    address: http://prometheus-operated.containership-core.svc.cluster.local:9090
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
| `aggregation` | false | `avg` | Prometheus aggregation function used in the query. See [the official documentation](https://prometheus.io/docs/prometheus/latest/querying/operators/#aggregation-operators) for more details. |
| `range` | false | `1m` | The time range over which to perform the Prometheus query. See [the official documentation](https://prometheus.io/docs/prometheus/latest/querying/basics/#range-vector-selectors) for more details. |
| `cpuMetricName` | false | `node_cpu` | CPU metric name to use in Prometheus query. |

#### Example
```yaml
apiVersion: cerebral.containership.io/v1alpha1
kind: AutoscalingPolicy
metadata:
  name: cpu-example-policy
spec:
  metric: cpu_percent_utilization
  metricConfiguration:
    cpuMetricName: node_cpu_seconds_total
  metricsBackend: prometheus
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
| `aggregation` | false | `avg` | Prometheus aggregation function used in the query. See [the official documentation](https://prometheus.io/docs/prometheus/latest/querying/operators/#aggregation-operators) for more details. |
| `range` | false | `1m` | The time range over which to perform the Prometheus query. See [the official documentation](https://prometheus.io/docs/prometheus/latest/querying/basics/#range-vector-selectors) for more details. |

#### Example
```yaml
apiVersion: cerebral.containership.io/v1alpha1
kind: AutoscalingPolicy
metadata:
  name: memory-example-policy
spec:
  metric: memory_percent_utilization
  metricConfiguration:
    aggregation: max
  metricsBackend: prometheus
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
| `query` | true | | Query that will be executed against the Prometheus API. See [the official documentation](https://prometheus.io/docs/prometheus/latest/querying/basics/) for more details. |

**Note:** In order for the query to target the hosts that are part of the AutoscalingGroup, the `query` configuration parameter should contain a templated `instance` clause such as:
```
instance=~'{{.PodIPsRegex}}'
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
    query: 100 - (avg(irate(node_cpu{mode='idle',instance=~'{{.PodIPsRegex}}'}[1m])) * 100)
  metricsBackend: prometheus
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
