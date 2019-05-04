#!/usr/bin/env bash

# This script updates all Kubernetes go modules
# It is borrowed and modified from:
# https://github.com/kubernetes/client-go/issues/581#issuecomment-481250973

[ $# -eq 1 ] || { echo "Supply new version number (no leading 'v')" >&2; exit 1; }

go get k8s.io/kubernetes@v$1 \
	k8s.io/cloud-provider@kubernetes-$1\
	k8s.io/api@kubernetes-$1\
	k8s.io/apimachinery@kubernetes-$1\
	k8s.io/apiserver@kubernetes-$1\
	k8s.io/apiextensions-apiserver@kubernetes-$1\
	k8s.io/csi-api@kubernetes-$1\
	k8s.io/kube-controller-manager@kubernetes-$1 \
	k8s.io/client-go@kubernetes-$1 \
	k8s.io/component-base@kubernetes-$1
