#!/usr/bin/env bash
# Generate Ed25519 PKCS#8 PEMs for API Server JWT: API profile vs Edge profile (separate Helm Secrets).
# Output layout matches Helm mounts: jwt.keys_dir (…/jwt) vs jwt.edge_keys_dir (…/jwt-edge).
#
# Usage:
#   ./generate-edge-jwt-key.sh [DEST_DIR]
# Default DEST_DIR: ./jwt-key-material (gitignored recommended)

set -euo pipefail

DEST="${1:-./jwt-key-material}"
mkdir -p "${DEST}/api" "${DEST}/edge"
ts="$(date -u +%Y-%m-%d)"
api_path="${DEST}/api/api-server-key-${ts}.key"
edge_path="${DEST}/edge/edge-server-key-${ts}.key"

openssl genpkey -algorithm ED25519 -out "${api_path}"
chmod 600 "${api_path}"
openssl genpkey -algorithm ED25519 -out "${edge_path}"
chmod 600 "${edge_path}"

echo "Wrote (kid = basename without .key):"
echo "  API:  ${api_path}   → kid api-server-key-${ts}"
echo "  Edge: ${edge_path}  → kid edge-server-key-${ts}"
echo
echo "Create two Kubernetes Secrets (namespace e.g. api-gateway):"
echo "  kubectl create secret generic api-gateway-jwt-api-keys -n api-gateway \\"
echo "    --from-file=$(basename "${api_path}")=${api_path} \\"
echo "    --dry-run=client -o yaml | kubectl apply -f -"
echo
echo "  kubectl create secret generic api-gateway-jwt-edge-keys -n api-gateway \\"
echo "    --from-file=$(basename "${edge_path}")=${edge_path} \\"
echo "    --dry-run=client -o yaml | kubectl apply -f -"
echo
echo "Helm (see values.cp.local.yaml):"
echo "  components.apiServer.jwt.apiKeysSecret: api-gateway-jwt-api-keys"
echo "  components.apiServer.jwt.edgeKeysSecret: api-gateway-jwt-edge-keys"
echo "Optional rotation pinning (Helm merged API Server config):"
echo "  components.apiServer.config.jwt.api_signing_kid: api-server-key-${ts}"
echo "  components.apiServer.config.jwt.edge_signing_kid: edge-server-key-${ts}"
