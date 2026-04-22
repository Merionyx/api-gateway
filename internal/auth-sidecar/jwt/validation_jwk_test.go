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

func TestJWTValidator_jwkToPublicKey_OKP_wrongCrv(t *testing.T) {
	t.Parallel()
	v := &JWTValidator{}
	_, err := v.jwkToPublicKey(&JWK{Kty: "OKP", Crv: "P-256", X: "abc"})
	if err == nil {
		t.Fatal("want unsupported curve")
	}
}

func TestJWTValidator_jwkToPublicKey_OKP_badX(t *testing.T) {
	t.Parallel()
	v := &JWTValidator{}
	_, err := v.jwkToPublicKey(&JWK{Kty: "OKP", Crv: "Ed25519", X: "not-valid-base64!!!"})
	if err == nil {
		t.Fatal("decode error")
	}
	shortX := base64.RawURLEncoding.EncodeToString([]byte{1, 2, 3})
	_, err = v.jwkToPublicKey(&JWK{Kty: "OKP", Crv: "Ed25519", X: shortX})
	if err == nil {
		t.Fatal("wrong size")
	}
}

func TestJWTValidator_jwkToPublicKey_RSA_badB64(t *testing.T) {
	t.Parallel()
	v := &JWTValidator{}
	_, err := v.jwkToPublicKey(&JWK{Kty: "RSA", N: "x", E: "y"})
	if err == nil {
		t.Fatal("N decode")
	}
	nB64 := base64.RawURLEncoding.EncodeToString([]byte{0x01})
	_, err = v.jwkToPublicKey(&JWK{Kty: "RSA", N: nB64, E: "!!!not-valid-b64$$$"})
	if err == nil {
		t.Fatal("E decode")
	}
}
