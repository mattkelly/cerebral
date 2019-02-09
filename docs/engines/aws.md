# Amazon Web Services (AWS) Engine

## Description
Cerebral is able to autoscale [AWS Auto Scaling Groups (ASGs)](https://docs.aws.amazon.com/autoscaling/ec2/userguide/AutoScalingGroup.html) by setting the `desired` node count.
It does not modify the min/max bounds on an ASG as set in AWS (which may have been performed through e.g. `kops`).
This means that a user should set the `min` and `max` fields on the [Cerebral AutoscalingGroup CR][cerebral-asg-cr] to match the AWS ASG bounds in order to have the expected behavior.

It is expected that when the AWS ASG scales up, the label(s) associated with the `nodeSelector` field in the AutoscalingGroup CR will be added to the new instance.
For example, the way to easily achieve this using `kops` is to define [`nodeLabels`](https://github.com/kubernetes/kops/blob/master/docs/labels.md) on the instance group that match the `nodeSelector`.

## Configuration
The AWS engine does not require any configuration in the AutoscalingEngine CR itself.
It is expected that the well-known AWS environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and optionally `AWS_REGION`) are pulled in through the main Cerebral Deployment.

The CR is still required in order to inform Cerebral about the existence of the engine, as shown below.

## Example
```yaml
apiVersion: cerebral.containership.io/v1alpha1
kind: AutoscalingEngine
metadata:
  name: aws
spec:
  type: aws
```

[cerebral-asg-cr]: ../../docs/custom_resource_definitions.md#autoscalinggroup
