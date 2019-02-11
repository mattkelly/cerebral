# Overview

This directory contains manifests for deploying the Containership engine.
These are similar to the manifests that are deployed when a [Containership Kubernetes Engine (CKE)](https://containership.io/containership-platform) cluster is launched with the Cerebral autoscaling plugin.
Normally, a user will not apply these manifests directly and instead go through Containership Cloud.
These are here for reference and for development purposes.

# File Structure

## 00-secret-cerebral-containership.yaml

This file contains a Secret referenced by the Containership authenticate with Containership Cloud.
See the [Containership engine documentation][containership-engine-docs] for more details on how to replace this value with a valid Containership API token.

## 10-deployment-cerebral-containership.yaml

This file contains the main Cerebral Deployment for running on Cerebral.

## 20-autoscaling-engine-containership.yaml

This file contains the AutoscalingEngine CustomResource that registers the Containership engine.
The configuration must be modified to include the proper `clusterID` and `organizationID`.
See the [Containership engine documentation][containership-engine-docs] for more details.

[containership-engine-docs]: ../../docs/engines/containership.md
