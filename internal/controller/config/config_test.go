package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func TestLoadConfig_FromRepoSample(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "configs", "controller", "config.prod.yaml")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.APIServer.Address != "gateway-api-server:19093" {
		t.Fatalf("APIServer.Address: %q", cfg.APIServer.Address)
	}
	if cfg.Tenant != "prod-cluster" {
		t.Fatalf("Tenant: %q", cfg.Tenant)
	}
	if len(cfg.Services.Static) == 0 {
		t.Fatal("expected services.static")
	}
}

func TestLoadConfig_MissingAPIServerAddress(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := `
server:
  http1_port: "1"
  http2_port: "2"
  grpc_port: "3"
  xds_port: "4"
kubernetes_discovery:
  enabled: true
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "api_server.address") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestLoadConfig_StaticEnvRequiresServices(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yaml")
	content := `
server:
  http1_port: "1"
  http2_port: "2"
  grpc_port: "3"
  xds_port: "4"
api_server:
  address: "x:1"
services:
  static:
    - name: root-svc
      upstream: "http://u:8080"
environments:
  - name: e1
    bundles:
      static:
        - name: b1
          repository: r
          ref: main
          path: p
    services:
      static: []
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "e1") || !strings.Contains(err.Error(), "services.static") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestFlattenYAMLStringMap_nested(t *testing.T) {
	out := flattenYAMLStringMap(map[string]interface{}{
		"app": map[string]interface{}{
			"kubernetes.io/name": "gw",
		},
	})
	if out["app.kubernetes.io/name"] != "gw" {
		t.Fatalf("got %#v", out)
	}
}
