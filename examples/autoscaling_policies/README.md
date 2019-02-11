# Overview

This directory contains a few basic examples of autoscaling policy CRs.
See [the documentation](../../docs/custom_resource_definitions.md#autoscalingpolicy) for more details on constructing your own.

# File Structure

## prometheus-cpu-utilization.yaml

This is an ASP that scales based on the CPU percent utilization as reported by Prometheus.

For more information, please refer to the [Prometheus metrics backend documentation](../../docs/metrics_backends/prometheus.md).

## prometheus-custom-cpu-utilization.yaml

This is an ASP that utilizes a custom Prometheus query.
For the purpose of demonstration, it performs the same query that could be achieved using the `cpu_percent_utilization` metric.

For more information, please refer to the [Prometheus metrics backend documentation](../../docs/metrics_backends/prometheus.md).

## kubernetes-cpu-allocation.yaml

This is an ASP that scales based on CPU percent allocation as reported by Kubernetes.

For more information, please refer to the [Kubernetes metrics backend documentation](../../docs/metrics_backends/kubernetes.md).

## influxdb-cpu-utilization

This is an ASP that scales based on the CPU percent utilization as reported by InfluxDB.

For more information, please refer to the [InfluxDB metrics backend documentation](../../docs/metrics_backends/influxdb.md).
