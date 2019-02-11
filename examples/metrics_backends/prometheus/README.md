# File Structure

## 00-metrics-backend-prometheus.yaml

This file contains a MetricsBackend CustomResource for registering the Prometheus backend with Cerebral.
This example assumes that Prometheus is running via the [Prometheus Operator](https://github.com/coreos/prometheus-operator) in the `default` namespace using the default port.

For more information, please refer to the [Prometheus metrics backend documentation](../../../docs/metrics_backends/prometheus.md).
