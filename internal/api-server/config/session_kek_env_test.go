package config

import (
	"os"
	"path/filepath"
	"testing"
)

// validBase64For32Bytes is standard base64 for 32 ASCII bytes (AES-256 key material shape).
const validBase64For32Bytes = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

func TestLoadConfig_SessionKEKFromEnvUnderscoredName(t *testing.T) {
	// Documents Helm/Kubernetes env API_SERVER_AUTH_SESSION_KEK_BASE64 → auth.session_kek_base64.
	t.Setenv("API_SERVER_AUTH_SESSION_KEK_BASE64", validBase64For32Bytes)

	dir := t.TempDir()
	path := filepath.Join(dir, "api-server.yaml")
	if err := os.WriteFile(path, []byte(minimalAPIServerYAMLForEnvTest()), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got := cfg.Auth.SessionKEKBase64; got != validBase64For32Bytes {
		t.Fatalf("SessionKEKBase64: got %q want %q", got, validBase64For32Bytes)
	}
}

func minimalAPIServerYAMLForEnvTest() string {
	return `
server:
  http_port: "8080"
  grpc_port: "19093"
  host: "0.0.0.0"
  cors:
    allow_origins: []
etcd:
  endpoints: ["http://127.0.0.1:2379"]
  dial_timeout: "5s"
  tls:
    enabled: false
jwt:
  keys_dir: "/tmp"
  issuer: "api-gateway-api-server"
contract_syncer:
  address: "syncer:19092"
grpc_registry:
  tls:
    enabled: false
grpc_contract_syncer_client:
  enabled: false
readiness: {}
leader_election:
  enabled: false
metrics_http: {}
idempotency: {}
telemetry: {}
auth:
  environment: production
`
}
