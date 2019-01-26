
# Containership Engine

## Description
The Containership engine integrates with [Containership Kubernetes Engine](https://containership.io/containership-platform) to provide cloud agnostic autoscaling on a multitude of cloud providers.

## Configuration
In order for the Containership engine to scale your cluster, you will need various configuration parameters.

| Field | Required | Type | Description |
| ----- | -------- | ---- | ----------- |
| `address` | true | string | The Containership provision API address. The parameter should be set to `https://provision.containership.io` |
| `tokenEnvVarName` | true | string | The environment variable name to use to get the Containership API token. |
| `organizationID` | true | string | The ID of the organization to which the cluster belongs. You can find this value on the "Organization Settings" page in Containership Cloud. |
| `clusterID` | true | string | The ID of the cluster that should be monitored and scaled. You can find this value in your URL once you click into a cluster in Containership Cloud. |

**Note:** You can acquire the Containership Cloud API key on the cluster itself by running the following command:
```
kubectl get secret containership-env-secret -n containership-core -o jsonpath='{.data.CONTAINERSHIP_CLOUD_CLUSTER_API_KEY}' | base64 -D
```

## Example
```yaml
apiVersion: cerebral.containership.io/v1alpha1
kind: AutoscalingEngine
metadata:
  name: containership
spec:
  type: containership
  configuration:
    address:              https://provision.containership.io
    tokenEnvVarName:      CONTAINERSHIP_CLOUD_CLUSTER_API_KEY
    organizationID:       15608402-d588-48c8-b326-db14b012d83e
    clusterID:            5253100f-dc07-462e-9b93-2fc2c0d5431f
```
