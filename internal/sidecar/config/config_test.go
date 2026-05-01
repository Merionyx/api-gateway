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
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func TestLoadConfig_FromRepoDev(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "configs", "sidecar", "config.dev.yaml")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Controller.Address == "" {
		t.Fatal("Controller.Address empty")
	}
	if cfg.Controller.Environment == "" {
		t.Fatal("Controller.Environment empty")
	}
	if cfg.JWT.JWKSURL == "" {
		t.Fatal("JWT.JWKSURL empty")
	}
	if cfg.JWT.ExpectedIssuer == "" {
		t.Fatal("JWT.ExpectedIssuer empty (defaults should apply)")
	}
	if cfg.JWT.ExpectedAudience == "" {
		t.Fatal("JWT.ExpectedAudience empty (defaults should apply)")
	}
}

func TestLoadConfig_NoFile_Defaults(t *testing.T) {
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.JWT.JWKSURL == "" {
		t.Fatal("expected default JWKS URL")
	}
	if cfg.JWT.ExpectedIssuer != "api-gateway-edge" || cfg.JWT.ExpectedAudience != "api-gateway-edge-http" {
		t.Fatalf("expected Edge claim defaults, got iss=%q aud=%q", cfg.JWT.ExpectedIssuer, cfg.JWT.ExpectedAudience)
	}
}
