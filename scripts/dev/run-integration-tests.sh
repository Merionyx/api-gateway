#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

IMAGE="${INTEGRATION_ETCD_IMAGE:-quay.io/coreos/etcd:v3.6.0}"
PORT="${INTEGRATION_ETCD_PORT:-2379}"
NAME="merionyx-integration-etcd-$$"

cleanup() {
  docker rm -f "$NAME" 2>/dev/null || true
}
trap cleanup EXIT

echo "Starting etcd ($IMAGE) as $NAME on port $PORT..."
docker run -d --name "$NAME" -p "${PORT}:2379" \
  -e ETCD_LISTEN_CLIENT_URLS="http://0.0.0.0:2379" \
  -e ETCD_ADVERTISE_CLIENT_URLS="http://127.0.0.1:${PORT}" \
  "$IMAGE"

echo "Waiting for etcd (HTTP /version on 127.0.0.1:$PORT)..."
ready=0
for _ in $(seq 1 120); do
  if curl -sf --connect-timeout 1 "http://127.0.0.1:${PORT}/version" >/dev/null; then
    ready=1
    break
  fi
  sleep 0.25
done
if [[ "$ready" -ne 1 ]]; then
  echo "etcd did not become healthy in time" >&2
  docker logs "$NAME" >&2 || true
  exit 1
fi

export INTEGRATION_ETCD_ENDPOINT="127.0.0.1:${PORT}"
echo "Running go test -tags=integration (INTEGRATION_ETCD_ENDPOINT=$INTEGRATION_ETCD_ENDPOINT)..."
go test -tags=integration ./test/integration/... -count=1 -timeout=5m -v "$@"
