package jwt

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestJWTValidator_jwkToPublicKey_Ed25519(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	v := &JWTValidator{}
	jwk := &JWK{
		Kty: "OKP",
		Crv: "Ed25519",
		X:   base64.RawURLEncoding.EncodeToString(pub),
	}
	got, err := v.jwkToPublicKey(jwk)
	if err != nil {
		t.Fatal(err)
	}
	gotPub, ok := got.(ed25519.PublicKey)
	if !ok || len(gotPub) != ed25519.PublicKeySize {
		t.Fatalf("unexpected key type %T", got)
	}
}

func TestJWTValidator_jwkToPublicKey_UnsupportedKty(t *testing.T) {
	v := &JWTValidator{}
	_, err := v.jwkToPublicKey(&JWK{Kty: "EC"})
	if err == nil {
		t.Fatal("expected error")
	}
}
