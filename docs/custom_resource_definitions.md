# Cerebral Custom Resource Definitions

Cerebral is comprised of various [Custom Resource Definitions (CRDs)](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions):

* [AutoscalingGroup](#autoscalinggroup)
* [AutoscalingPolicy](#autoscalingpolicy)
* [AutoscalingEngine](#autoscalingengine)
* [MetricsBackend](#metricsbackend)

The relationships between them are as follows:
* `AutoscalingGroup` references __one__ `AutoscalingEngine` and __one or more__ `AutoscalingPolicies`.
* `AutoscalingPolicy` references __one__ `MetricsBackend`.
* `AutoscalingEngine` describes how to communicate with __one__ autoscaling provider.
* `MetricsBackend` describes how to communicate with  __one__ metrics source.

All CRDs can be deployed using the `examples/common` directory:

```
kubectl apply -f examples/common
```

### AutoscalingGroup

An `AutoscalingGroup` is defined as a group of nodes that exists within the Kubernetes cluster, and has the ability to be scaled independently of other nodes in the cluster.

The manifest for an AutoscalingGroup is available within the [prerequisite manifests][examples-prereqs].

#### Example

Examples are available in the [examples directory][examples-autoscaling-groups].

#### Fields

| Path | Required | Type | Description |
|------|----------|------|-------------|
| `spec.nodeSelector` | true | object | Set of key / value label pairs which are logically ANDed together, and are responsible for selecting the nodes that comprise the `AutoscalingGroup` |
| `spec.policies` | true | string | List of `AutoscalingPolicy` names applied to the `AutoscalingGroup` |
| `spec.cooldownPeriod` | true | number | Number of seconds to disable scaling events after a scaling action takes place |
| `spec.suspended` | true | boolean | Flag indicating whether scaling actions are allowed to take place |
| `spec.minNodes` | true | number | Minimum number of nodes in the group |
| `spec.maxNodes` | true | number | Maximum number of nodes in the group |
| `spec.engine` | true | string | Associated `AutoscalingEngine` used to change capacity of the `AutoscalingGroup` |
| `spec.scalingStrategy.scaleUp` | false | string | String representation of the `ScalingStrategy` to use when triggering a scale up operation. Default value provided by the associated `AutoscalingEngine`. |
| `spec.scalingStrategy.scaleDown` | false | string | String representation of the `ScalingStrategy` to use when triggering a scale down operation. Default value provided by the associated `AutoscalingEngine`. |
| `status.lastUpdateTime` | false | string | Timestamp representing the last time the `AutoscalingGroup` triggered a scale event |

#### Notes

**Important**: The set of nodes selected by each `nodeSelector` must be disjoint from the sets of nodes selected by all other selectors for other `AutoscalingGroups`.
Otherwise, a single node would belong to multiple `AutoscalingGroups`.

### AutoscalingPolicy

An `AutoscalingPolicy` is defined as a list of thresholds, responsible for triggering one or more `AutoscalingGroups` to scale either up or down based on the returned metric value from the `MetricsBackend`.

The manifest for an AutoscalingPolicy is available within the [prerequisite manifests][examples-prereqs].

#### Example

Examples are available in the [examples directory][examples-autoscaling-policies].
See also [documentation for specific metrics backends][docs-metrics-backends].

#### Fields

| Path | Required | Type | Description |
|------|----------|------|-------------|
| `spec.metricsBackend` | true | string | `MetricsBackend` name associated with the `AutoscalingPolicy` |
| `spec.metric` | true | string | String representation of the target metric to monitor. Available values are provided by the given `MetricsBackend`. |
| `spec.metricConfiguration` | false | object | Arbitrary configuration object used to configure the metric polling. See [metrics configuration](#metrics-configuration). |
| `spec.policy.scaleUp` | false | object | Policy object containing parameters used when scaling an `AutoscalingGroup` up |
| `spec.policy.scaleUp.threshold` | true | number | Numerical representation of the threshold at which when the comparison evaluates to true, the associated `AutoscalingGroups` should scale up
| `spec.policy.scaleUp.comparisonOperator` | true | string | The comparison operator to use when comparing the `MetricsBackend` metric value to the `threshold` value. Allowed values are `>`, `<`, `>=`, `<=`, `==`, `!=` |
| `spec.policy.scaleUp.adjustmentType` | true | string | Method by which to add capacity to the `AutoscalingGroup`. Absolute represents an exact number of nodes, whereas percent represents a percentage (rounded up to the nearest whole number) of nodes in the pool. |
| `spec.policy.scaleUp.adjustmentValue` | true | number | Numerical representation of the number of nodes to scale the `AutoscalingGroup` up by determined by the `adjustmentType` |
| `spec.policy.scaleDown` | false | object | Policy object containing parameters used when scaling an `AutoscalingGroup` down |
| `spec.policy.scaleDown.threshold` | true | number | Numerical representation of the threshold at which when thhe comparison evaluates to true, the associated `AutoscalingGroups` should scale down
| `spec.policy.scaleDown.comparisonOperator` | true | string | The comparison operator to use when comparing the `MetricsBackend` metric value to the `threshold` value. Allowed values are `>`, `<`, `>=`, `<=`, `==`, `!=` |
| `spec.policy.scaleDown.adjustmentType` | true | string | Method by which to add capacity to the `AutoscalingGroup`. Absolute represents an exact number of nodes, whereas percent represents a percentage (rounded up to the nearest whole number) of nodes in the pool. |
| `spec.policy.scaleDown.adjustmentValue` | true | number | Numerical representation of the number of nodes to scale the `AutoscalingGroup` down by determined by the `adjustmentType` |
| `spec.pollInterval` | true | number | Number of seconds between polling the associated `MetricsBackend` |
| `spec.samplePeriod` | true | number | Number of seconds the `AutoscalingPolicy` must alert the threshold before the policy triggers a scale up or scale down action |

#### Notes

The `AutoscalingPolicy` can be thought of as a mathematical comparison defined as: `returnedMetricValue` `spec.policy{scaleUp,scaleDown}.comparisonOperator` `spec.policy{scaleUp,scaleDown}.threshold`.

If the comparison evaluates to `true`, the `AutoscalingPolicy` is said to "alert".
A scale request is generated only if the threshold is breached for at least the `samplePeriod`.

* The `AutoscalingPolicy` is not required to implement both a `scaleUp` and `scaleDown` policy definition.
* Since `AutoscalingGroup`s are often homogeneous, a `scalingStrategy` is often only used in conjunction with a `scaleDown` policy.
* If the group were heterogeneous, the `scaleUp` policy could in theory pick a node and add capacity of the same instance type.

### AutoscalingEngine

An `AutoscalingEngine` is defined as the system responsible for adding or removing capacity to the Kubernetes cluster.

The manifest for an AutoscalingEngine is available within the [prerequisite manifests][examples-prereqs].

#### Example

Example AutoscalingEngine manifests are available [in the examples directory][examples-autoscaling-engines].

#### Fields

| Path | Required | Type | Description |
|------|----------|------|-------------|
| `spec.type` | true | string | Type of engine |
| `spec.configuration` | true | object | Type-dependent configuration information for the engine |

### MetricsBackend

A `MetricsBackend` is defined as a source from which the cluster autoscaler will poll for metrics, returning a raw metric value to compare against the thresholds defined in the `AutoscalingPolicies` in order to make scaling decisions.
See the relevant `AutoscalingPolicy` fields [here](#autoscaling-policy-fields).

The manifest for a MetricsBackend is available within the [prerequisite manifests][examples-prereqs].

#### Example

Example MetricsBackend manifests are available [in the examples directory][examples-metrics-backends].

#### Fields

| Path | Required | Type | Description |
|------|----------|------|-------------|
| `spec.type` | true | string | Type of metrics backend |
| `spec.configuration` | true | object | Type-dependent configuration information for the metrics backend, i.e. information required to communicate with it |

##### Metric Configuration

The `MetricsBackend` is required to expose a list of well-defined metrics which the user can leverage in an `AutoscalingPolicy`.
Each metric may expose a different set of configurable parameters.
For Prometheus this looks like:

###### `cpu_percent_utilization`

Monitor the average CPU across an `AutoscalingGroup`.
Configuration is as follows:

| Key | Required | Type | Description |
|-----|----------|------|-------------|
| `aggregation` | false | string | Aggregation function to apply the metric (default `avg`) |
| `range` | false | string | Historical range over which to aggregate the metric (default `1m`) |
| `cpuMetricName` | false | string | Name of the CPU metric to fetch from Prometheus (default `node_cpu`) |

Note that [#48](https://github.com/containership/cerebral/issues/48) should remove the need to have a `cpuMetricName` configuration key.

###### `memory_percent_utilization`

Monitor the average memory across an `AutoscalingGroup`.
Configuration is as follows:

| Key | Required | Type | Description |
|-----|----------|------|-------------|
| `aggregation` | false | string | Aggregation function to apply the metric (default `avg`) |
| `range` | false | string | Historical range over which to aggregate the metric (default `1m`) |

###### `custom`

Monitor a custom metric across an `AutoscalingGroup`.
Configuration is as follows:

| Key | Required | Type | Description |
|-----|----------|------|-------------|
| `queryTemplate` | true | string | String template used to build the query |

[examples-prereqs]: ../examples/common/00-prereqs.yaml
[examples-autoscaling-engines]: ../examples/engines
[examples-autoscaling-groups]: ../examples/autoscaling_groups
[examples-autoscaling-policies]: ../examples/autoscaling_policies
[examples-metrics-backends]: ../examples/metrics_backends
[docs-metrics-backends]: ../docs/metrics_backends
