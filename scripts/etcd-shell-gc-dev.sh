#!/bin/bash
# Source this file to set etcd environment variables
# Usage: source scripts/etcd-env.sh

export ETCDCTL_API=3
export ETCDCTL_ENDPOINTS=https://localhost:2479,https://localhost:3479,https://localhost:4479
export ETCDCTL_CACERT=$(pwd)/secrets/certs/etcd/ca.pem
export ETCDCTL_CERT=$(pwd)/secrets/certs/etcd/client.pem
export ETCDCTL_KEY=$(pwd)/secrets/certs/etcd/client-key.pem

echo
echo "✓ etcd environment variables set"
