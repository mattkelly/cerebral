# File Structure

## 00-secret-cerebral-aws.yaml

This file contains a Secret referenced by the AWS Cerebral deployment in order to authenticate with AWS.

The dummy values for `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` must be replace with proper base64-encoded values.

## 10-deployment-cerebral-aws.yaml

This file contains the main Cerebral Deployment for running on AWS.

## 20-autoscaling-engine-aws.yaml

This file contains the AutoscalingEngine CustomResource that registers the AWS engine.
