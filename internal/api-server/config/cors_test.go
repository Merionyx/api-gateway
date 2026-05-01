package config

import (
	"strings"
	"testing"
)

func TestNormalizeCORSAllowOrigins(t *testing.T) {
	t.Parallel()
	in := []string{" http://a ", "", "http://b", " http://a "}
	got := NormalizeCORSAllowOrigins(in)
	if len(got) != 2 || got[0] != "http://a" || got[1] != "http://b" {
		t.Fatalf("got %#v", got)
	}
}

func TestValidateServerCORS_wildcardRules(t *testing.T) {
	t.Parallel()
	err := ValidateServerCORS(CORSConfig{
		AllowOrigins:          []string{"*"},
		InsecureAllowWildcard: false,
	}, "development")
	if err == nil || !strings.Contains(err.Error(), "insecure_allow_wildcard") {
		t.Fatalf("want insecure_allow_wildcard error, got %v", err)
	}
	err = ValidateServerCORS(CORSConfig{
		AllowOrigins:          []string{"*"},
		InsecureAllowWildcard: true,
	}, "production")
	if err == nil || !strings.Contains(err.Error(), "wildcard") {
		t.Fatalf("want production wildcard error, got %v", err)
	}
	if err := ValidateServerCORS(CORSConfig{
		AllowOrigins:          []string{"*"},
		InsecureAllowWildcard: true,
	}, "development"); err != nil {
		t.Fatal(err)
	}
	err = ValidateServerCORS(CORSConfig{
		AllowOrigins:          []string{"*", "http://a"},
		InsecureAllowWildcard: true,
	}, "development")
	if err == nil || !strings.Contains(err.Error(), "mix") {
		t.Fatalf("want mix error, got %v", err)
	}
}

func TestApplyCORSDevDefaults(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Auth: AuthConfig{Environment: "production"},
		Server: ServerConfig{
			CORS: CORSConfig{AllowOrigins: []string{}},
		},
	}
	ApplyCORSDevDefaults(cfg)
	if len(cfg.Server.CORS.AllowOrigins) != 0 {
		t.Fatalf("prod empty should stay empty, got %#v", cfg.Server.CORS.AllowOrigins)
	}
	cfg2 := &Config{
		Auth: AuthConfig{Environment: "development"},
		Server: ServerConfig{
			CORS: CORSConfig{AllowOrigins: nil},
		},
	}
	ApplyCORSDevDefaults(cfg2)
	if len(cfg2.Server.CORS.AllowOrigins) != len(DevCORSAllowOrigins()) {
		t.Fatalf("dev got %d", len(cfg2.Server.CORS.AllowOrigins))
	}
}
