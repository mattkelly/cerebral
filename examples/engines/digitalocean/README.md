# File Structure

## 00-secret-cerebral-digitalocean.yaml

This file contains a Secret referenced by the DigitalOcean Cerebral deployment in order to authenticate with DigitalOcean.

## 10-deployment-cerebral-digitalocean.yaml

This file contains the main Cerebral Deployment for running on DigitalOcean.

## 20-autoscaling-engine-digitalocean.yaml

This file contains the AutoscalingEngine CustomResource that registers the DigitalOcean engine.

The `clusterID` and `nodePoolLabelKey` should be replaced with valid values.
See the [DigitalOcean engine documentation](../../docs/engines/digitalocean.md) for more information.

