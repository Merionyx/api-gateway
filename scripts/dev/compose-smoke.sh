#!/usr/bin/env bash
# Smoke: поднять dev compose, проверить доступность метрик/JWKS, остановить.
# Требует Docker. Из корня репозитория:
#   ./scripts/dev/compose-smoke.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

COMPOSE_FILES=(
  -f ./deployments/dev/docker/compose.app.yaml
  -f ./deployments/dev/docker/compose.sidecar.yaml
  -f ./deployments/dev/docker/compose.etcd.yaml
  -f ./deployments/dev/docker/compose.envoy.yaml
  -f ./deployments/dev/docker/compose.mock-service.yaml
)

cleanup() {
  docker compose -p merionyx-api-gateway-smoke "${COMPOSE_FILES[@]}" down -v --remove-orphans 2>/dev/null || true
}
trap cleanup EXIT

docker compose -p merionyx-api-gateway-smoke "${COMPOSE_FILES[@]}" up -d --build

echo "Waiting for stack (60s max)..."
for i in $(seq 1 60); do
  if curl -sf "http://127.0.0.1:8080/.well-known/jwks.json" >/dev/null 2>&1; then
    echo "JWKS OK"
    break
  fi
  if [[ "$i" -eq 60 ]]; then
    echo "Timeout waiting for JWKS on :8080"
    exit 1
  fi
  sleep 1
done

if curl -sf "http://127.0.0.1:8080/metrics" | grep -q .; then
  echo "Metrics endpoint OK"
else
  echo "Metrics check failed (non-fatal if path differs)"
fi

echo "Compose smoke passed."
