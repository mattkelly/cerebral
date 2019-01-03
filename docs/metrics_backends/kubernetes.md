# Kubernetes Metrics Backend

## Description
The Kubernetes metrics backend interfaces with the Kubernetes API to expose allocation metrics gathered by querying for pod resource requests.

## Configuration
Since the Kubernetes metrics backend implementation uses an in-cluster configuration, no additional configuration is required.

## Example
```yaml
apiVersion: cerebral.containership.io/v1alpha1
kind: MetricsBackend
metadata:
  name: kubernetes
spec:
  type: kubernetes
```

## Available Metrics
* [CPU Percent Allocation](#cpu-percent-allocation)
* [Memory Percent Allocation](#memory-percent-allocation)
* [Ephemeral Storage Percent Allocation](#ephemeral-storage-percent-allocation)
* [Pod Percent Allocation](#pod-percent-allocation)

### CPU Percent Allocation

#### Description
Returns the percent of allocated CPUs across the nodes in the autoscaling group by summing the total CPU requests of the pods, and dividing by the nodes' total allocatable CPUs.

#### Metric
`cpu_percent_allocation`

#### Configuration
No configuration is available for this metric type.

#### Example
```yaml
apiVersion: cerebral.containership.io/v1alpha1
kind: AutoscalingPolicy
metadata:
  name: cpu-example-policy
spec:
  metric: cpu_percent_allocation
  metricConfiguration: {}
  metricsBackend: kubernetes
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

### Memory Percent Allocation

#### Description
Returns the percent of allocated memory across the nodes in the autoscaling group by summing the total memory requests of the pods, and dividing by the nodes' total allocatable memory.

#### Metric
`memory_percent_allocation`

#### Configuration
No configuration is available for this metric type.

#### Example
```yaml
apiVersion: cerebral.containership.io/v1alpha1
kind: AutoscalingPolicy
metadata:
  name: memory-example-policy
spec:
  metric: memory_percent_allocation
  metricConfiguration: {}
  metricsBackend: kubernetes
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

### Ephemeral Storage Percent Allocation

#### Description
Returns the percent of allocated ephemeral storage across the nodes in the autoscaling group by summing the total ephemeral storage requests of the pods, and dividing by the nodes' total allocatable ephemeral storage.

#### Metric
`ephemeral_storage_percent_allocation`

#### Configuration
No configuration is available for this metric type.

#### Example
```yaml
apiVersion: cerebral.containership.io/v1alpha1
kind: AutoscalingPolicy
metadata:
  name: ephemeral-storage-example-policy
spec:
  metric: ephemeral_storage_percent_allocation
  metricConfiguration: {}
  metricsBackend: kubernetes
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

### Pod Percent Allocation

#### Description
Returns the percent of allocated pods across the nodes in the autoscaling group by summing the total number of pods running on the nodes, and dividing by the nodes' total allocatable pods.

#### Metric
`pod_percent_allocation`

#### Configuration
No configuration is available for this metric type.

#### Example
```yaml
apiVersion: cerebral.containership.io/v1alpha1
kind: AutoscalingPolicy
metadata:
  name: pod-example-policy
spec:
  metric: pod_percent_allocation
  metricConfiguration: {}
  metricsBackend: kubernetes
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
