package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/shared/etcd"
)

func TestLoadConfig_MinimalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `
server:
  grpc_port: "19092"
  host: "0.0.0.0"
etcd:
  endpoints:
    - "http://127.0.0.1:2379"
  dial_timeout: 3s
api_server:
  address: "localhost:19093"
repositories: []
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.APIServer.Address != "localhost:19093" {
		t.Fatalf("APIServer.Address: %q", cfg.APIServer.Address)
	}
	if cfg.Etcd.DialTimeout != 3*time.Second {
		t.Fatalf("DialTimeout: %v", cfg.Etcd.DialTimeout)
	}
}

func TestApplyContractSyncerDefaults_TLSFromHTTPS(t *testing.T) {
	c := &Config{
		Server: ServerConfig{GRPCPort: "", Host: ""},
		Etcd: etcd.EtcdConfig{
			Endpoints:   []string{"https://etcd:2379"},
			DialTimeout: 0,
			TLS:         etcd.EtcdTLSConfig{Enabled: false},
		},
	}
	applyContractSyncerDefaults(c)
	if c.Server.GRPCPort != "19092" {
		t.Fatalf("GRPCPort: %q", c.Server.GRPCPort)
	}
	if c.Server.Host != "0.0.0.0" {
		t.Fatalf("Host: %q", c.Server.Host)
	}
	if c.Etcd.DialTimeout != 5*time.Second {
		t.Fatalf("DialTimeout: %v", c.Etcd.DialTimeout)
	}
	if !c.Etcd.TLS.Enabled {
		t.Fatal("expected TLS enabled from https endpoint")
	}
}
