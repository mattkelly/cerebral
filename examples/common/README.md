# File Structure

## 00-prereqs.yaml

This file contains the prerequisites necessary to run Cerebral:

- CustomResourceDefinitions (more info available on each is available [here](../../docs/custom_resource_definitions.md))
- `cerebral` ServiceAccount in `kube-system` Namespace
- RBAC rules to grant permissions to the `cerebral` ServiceAccount
