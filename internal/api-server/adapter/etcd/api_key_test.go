package etcd

import (
	"errors"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
)

func TestSHA256DigestHexFromSecret(t *testing.T) {
	t.Parallel()
	got := SHA256DigestHexFromSecret("hello")
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Fatalf("digest: %q", got)
	}
}

func TestConstantTimeDigestHexEqual(t *testing.T) {
	t.Parallel()
	a := SHA256DigestHexFromSecret("x")
	b := SHA256DigestHexFromSecret("x")
	ok, err := ConstantTimeDigestHexEqual(a, b)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	ok, err = ConstantTimeDigestHexEqual(a, SHA256DigestHexFromSecret("y"))
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestConstantTimeDigestHexEqualInvalid(t *testing.T) {
	t.Parallel()
	_, err := ConstantTimeDigestHexEqual("not-hex", SHA256DigestHexFromSecret("z"))
	if !errors.Is(err, ErrInvalidDigestHex) {
		t.Fatalf("got %v", err)
	}
}

func TestAPIKeyRepositoryKeyForDigest(t *testing.T) {
	t.Parallel()
	r := NewAPIKeyRepository(nil, "/api-gateway/api-server/auth/v1")
	_, err := r.keyForDigest("g" + SHA256DigestHexFromSecret("a")[1:])
	if !errors.Is(err, ErrInvalidDigestHex) {
		t.Fatalf("want invalid digest, got %v", err)
	}
	d := SHA256DigestHexFromSecret("secret")
	key, err := r.keyForDigest(d)
	if err != nil {
		t.Fatal(err)
	}
	want := "/api-gateway/api-server/auth/v1/api-keys/sha256/" + d
	if key != want {
		t.Fatalf("key %q want %q", key, want)
	}
}

func TestBootstrapPutDevelopment_Disabled(t *testing.T) {
	t.Parallel()
	r := NewAPIKeyRepository(nil, DefaultAuthEtcdKeyPrefix)
	_, err := r.BootstrapPutDevelopment(t.Context(), false, "k", mustAPIKeyValue(t))
	if !errors.Is(err, ErrBootstrapAPIKeyDisabled) {
		t.Fatalf("got %v", err)
	}
}

func mustAPIKeyValue(t *testing.T) kvvalue.APIKeyValue {
	t.Helper()
	return kvvalue.APIKeyValue{
		Algorithm:    "sha256",
		Roles:        []string{"ci"},
		Scopes:       []string{"registry:read"},
		RecordFormat: kvvalue.DefaultAPIKeyRecordFormat,
	}
}
