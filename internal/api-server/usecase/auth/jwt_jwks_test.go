package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestJWTUseCase_GetJWKS_GetSigningKeys_generated(t *testing.T) {
	t.Parallel()
	uc, err := NewJWTUseCase(t.TempDir(), "iss", "")
	if err != nil {
		t.Fatal(err)
	}
	jwks, err := uc.GetJWKS(context.Background())
	if err != nil || len(jwks.Keys) != 1 {
		t.Fatalf("jwks: %v len=%d", err, len(jwks.Keys))
	}
	if jwks.Keys[0].Kty != "OKP" {
		t.Fatalf("want EdDSA JWK, got %q", jwks.Keys[0].Kty)
	}
	keys := uc.GetSigningKeys(context.Background())
	if len(keys) != 1 || !keys[0].Active {
		t.Fatalf("signing keys: %#v", keys)
	}
}

func TestJWTUseCase_GetJWKS_rsaKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
	keyPath := filepath.Join(dir, "rsa-signing.key")
	if err := os.WriteFile(keyPath, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	uc, err := NewJWTUseCase(dir, "iss", "")
	if err != nil {
		t.Fatal(err)
	}
	jwks, err := uc.GetJWKS(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var foundRSA bool
	for _, k := range jwks.Keys {
		if k.Kty == "RSA" && k.Alg == "RS256" {
			foundRSA = true
			break
		}
	}
	if !foundRSA {
		t.Fatalf("no RSA JWK in %#v", jwks.Keys)
	}
}
