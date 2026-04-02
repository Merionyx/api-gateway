#!/bin/bash
# Source this file to set etcd environment variables
# Usage: source scripts/etcd-env.sh

export ETCDCTL_API=3
export ETCDCTL_ENDPOINTS=https://etcd.default:61403
export ETCDCTL_CACERT=$(pwd)/secrets/k8s-etcd/ca.crt
export ETCDCTL_CERT=$(pwd)/secrets/k8s-etcd/tls.crt
export ETCDCTL_KEY=$(pwd)/secrets/k8s-etcd/tls.key

echo
echo "✓ etcd environment variables set"
