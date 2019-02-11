# Overview

This directory contains a few basic examples of autoscaling group CRs.
See [the documentation](../../docs/custom_resource_definitions.md#autoscalinggroup) for more details on constructing your own.

# File Structure

## containership-prometheus.yaml

This is an ASG similar to one that might be created through Containership Cloud when using Prometheus as a metrics backend.

## kops-kubernetes.yaml

This is an ASG that could be used for a worker node pool created by `kops` on AWS with the following [`nodeLabels`](https://github.com/kubernetes/kops/blob/master/docs/labels.md) configuration:

```
...
spec:
  nodeLabels:
    node-pool: workers
...
```

Note that `minNodes` and `maxNodes` should match the AutoScaling Group in AWS to avoid unexpected behavior.
See the [AWS engine documentation](../../docs/engines/aws.md) for more details.
