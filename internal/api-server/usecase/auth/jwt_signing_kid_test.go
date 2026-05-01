package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveSigningKeyID(t *testing.T) {
	t.Parallel()
	keys := map[string]*KeyPair{"a": {Kid: "a"}, "b": {Kid: "b"}}
	kid, err := resolveSigningKeyID("api", keys, "b", "a")
	if err != nil || kid != "b" {
		t.Fatalf("got %q %v", kid, err)
	}
	_, err = resolveSigningKeyID("api", keys, "missing", "a")
	if err == nil {
		t.Fatal("expected error for unknown kid")
	}
}

func TestLoadKeyDirectory_nonexistentDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	missing := filepath.Join(dir, "does-not-exist")
	keys, newest, _, err := loadKeyDirectory(missing)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 || newest != "" {
		t.Fatalf("got keys=%d newest=%q", len(keys), newest)
	}
}

func TestNewJWTUseCase_APISigningKidPinned(t *testing.T) {
	dir := t.TempDir()
	edgeDir := filepath.Join(dir, "edge")
	if err := os.MkdirAll(edgeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeEd25519KeyPEM(t, filepath.Join(dir, "api-old.key"))
	writeEd25519KeyPEM(t, filepath.Join(dir, "api-new.key"))
	oldT := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	newT := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(filepath.Join(dir, "api-old.key"), oldT, oldT); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(dir, "api-new.key"), newT, newT); err != nil {
		t.Fatal(err)
	}
	writeEd25519KeyPEM(t, filepath.Join(edgeDir, "edge-only.key"))

	cfg := jwtTestCfg(t, dir)
	cfg.APISigningKid = "api-old"
	uc, err := NewJWTUseCase(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if uc.apiActiveKeyID != "api-old" {
		t.Fatalf("api active kid: got %q", uc.apiActiveKeyID)
	}
	if uc.edgeActiveKeyID != "edge-only" {
		t.Fatalf("edge active kid: got %q", uc.edgeActiveKeyID)
	}
	for _, sk := range uc.GetSigningKeys(context.Background()) {
		if sk.Kid == "api-old" && !sk.Active {
			t.Fatalf("api-old should be active, got %#v", sk)
		}
		if sk.Kid == "api-new" && sk.Active {
			t.Fatalf("api-new should not be active")
		}
	}
}

func writeEd25519KeyPEM(t *testing.T, path string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
}
