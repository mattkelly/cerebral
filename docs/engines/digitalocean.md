# DigitalOcean Engine

## Limitations
Cerebral Autoscaler works for DigitalOcean Kubernetes clusters but treats the whole cluster as one Autoscaling Group.
This is due to DigitalOcean Kubernetes not automatically labeling nodes with their node pool information.
This limitation will hopefully be short lived, and a fix for it is in progress.
We are monitoring the status and progress on this issue [here](https://github.com/containership/cerebral/issues/52).

## Description
The DigitalOcean engine watches for scale events, which can be triggered if a node pool is not in bounds of the Autoscaling Group, or when threshold events are triggered from [Metrics Backends](/docs/metrics_backends).

## Configuration
In order for the DigitalOcean engine to scale your cluster, you will need to get the cluster ID and API token.
You can find the cluster ID in the URL when looking at the cluster through the DigitalOcean dashboard.
Finally, you can acquire an API token through the "Account" page on the DigitalOcean dashboard.
**Note:** The token that is used will need to have both read and write privileges to be able to scale the node pool.

| Field | Required | Type | Description |
| ----- | -------- | ---- | ----------- |
| `tokenEnvVarName` | true | string | The environment variable name to use to get the DigitalOcean API token. |
| `clusterID` | true | string | The ID of the cluster that should be monitored and scaled. |

## Example
```yaml
apiVersion: cerebral.containership.io/v1alpha1
kind: AutoscalingEngine
metadata:
  name: digitalocean
spec:
  type: digitalocean
  configuration:
    tokenEnvVarName:      DO_TOKEN
    clusterID:            5253100f-dc07-462e-9b93-2fc2c0d5431f
```
