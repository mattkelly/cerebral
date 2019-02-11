# Overview

This `examples` directory contains example manifests for getting started with Cerebral.
Since these are example manifests, they may reference `latest` tags and as such should not be used directly in production.

Where it matters, filenames are prefixed with an integer in order to ensure a sane apply order when using `kubectl apply` on an entire directory.

# File Structure

## `common`

This directory contains common manifests, such as the prerequisite CRDs, that should be applied before applying any other manifests.

## `engines`

This directory contains the manifests required to register and run the various autoscaling engines that Cerebral supports.
Because all engines are currently in-tree and they may require configuration on the Cerebral deployment itself (see #45), the main Cerebral deployment manifest for each engine/provider is also available in each subdirectory.

Depending on the engine, these manifests may require some manual modifications to work within a given cluster configuration.
Such manifests are clearly called out in individual READMEs.

## `metrics_backends`

This directory contains the manifests required to register and run the various metrics backends that Cerebral supports.

Depending on the metrics backend, these manifests may require some manual modifications to work within a given cluster configuration.
Such manifests are clearly called out in individual READMEs.

## `autoscaling_groups`

This directory contains example autoscaling group CRs.
Users should modify them to fit their specific requirements.

## `autoscaling_policies`

This directory contains example autoscaling policy CRs.
Users should modify them to fit their specific requirements.
