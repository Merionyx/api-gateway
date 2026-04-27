package config

import (
	"path/filepath"
	"runtime"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/api-server/config -> repo root is ../../..
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func TestLoadConfig_FromRepoSample(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "configs", "api-server", "config.yaml")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Server.HTTPPort != "8080" {
		t.Fatalf("HTTPPort: got %q", cfg.Server.HTTPPort)
	}
	if cfg.Server.GRPCPort != "19093" {
		t.Fatalf("GRPCPort: got %q", cfg.Server.GRPCPort)
	}
	if cfg.JWT.Issuer != "api-gateway-api-server" {
		t.Fatalf("JWT.Issuer: got %q", cfg.JWT.Issuer)
	}
	if cfg.ContractSyncer.Address != "gateway-contract-syncer:19092" {
		t.Fatalf("ContractSyncer.Address: got %q", cfg.ContractSyncer.Address)
	}
	if len(cfg.Etcd.Endpoints) != 3 {
		t.Fatalf("Etcd.Endpoints: len %d", len(cfg.Etcd.Endpoints))
	}
	if !cfg.LeaderElection.Enabled {
		t.Fatal("LeaderElection.Enabled expected true")
	}
	if cfg.Readiness.RequireContractSyncer {
		t.Fatal("Readiness.RequireContractSyncer expected false from sample config")
	}
	if cfg.Auth.InteractiveAccessTokenTTL != DefaultInteractiveAccessTokenTTL {
		t.Fatalf("InteractiveAccessTokenTTL: got %s", cfg.Auth.InteractiveAccessTokenTTL)
	}
	if cfg.Auth.InteractiveAccessTokenMaxTTL != DefaultInteractiveAccessTokenMaxTTL {
		t.Fatalf("InteractiveAccessTokenMaxTTL: got %s", cfg.Auth.InteractiveAccessTokenMaxTTL)
	}
	if cfg.Auth.InteractiveRefreshTokenTTL != DefaultInteractiveRefreshTokenTTL {
		t.Fatalf("InteractiveRefreshTokenTTL: got %s", cfg.Auth.InteractiveRefreshTokenTTL)
	}
	if cfg.Auth.InteractiveRefreshTokenMaxTTL != DefaultInteractiveRefreshTokenMaxTTL {
		t.Fatalf("InteractiveRefreshTokenMaxTTL: got %s", cfg.Auth.InteractiveRefreshTokenMaxTTL)
	}
	if len(cfg.Server.CORS.AllowOrigins) != 0 {
		t.Fatalf("sample prod config should keep server.cors.allow_origins empty (operator-filled), got %#v", cfg.Server.CORS.AllowOrigins)
	}
}

func TestLoadConfig_NoFile_Defaults(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Server.HTTPPort != "8080" || cfg.Server.Host != "localhost" {
		t.Fatalf("server defaults: %+v", cfg.Server)
	}
	if cfg.JWT.Issuer != "api-gateway-api-server" {
		t.Fatalf("JWT default issuer: %q", cfg.JWT.Issuer)
	}
	if cfg.Auth.InteractiveAccessTokenMaxTTL != DefaultInteractiveAccessTokenMaxTTL {
		t.Fatalf("access max ttl default: %s", cfg.Auth.InteractiveAccessTokenMaxTTL)
	}
	if cfg.Auth.InteractiveRefreshTokenTTL != DefaultInteractiveRefreshTokenTTL {
		t.Fatalf("refresh ttl default: %s", cfg.Auth.InteractiveRefreshTokenTTL)
	}
	if cfg.Auth.InteractiveRefreshTokenMaxTTL != DefaultInteractiveRefreshTokenMaxTTL {
		t.Fatalf("refresh max ttl default: %s", cfg.Auth.InteractiveRefreshTokenMaxTTL)
	}
	if !cfg.LeaderElection.Enabled {
		t.Fatal("leader election should default to enabled")
	}
	if cfg.Readiness.RequireContractSyncer {
		t.Fatal("readiness.require_contract_syncer should default to false")
	}
	if len(cfg.Server.CORS.AllowOrigins) == 0 {
		t.Fatal("no config file: expected default server.cors.allow_origins for browser dev")
	}
}
