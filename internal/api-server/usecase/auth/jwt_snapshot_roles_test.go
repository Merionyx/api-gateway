package auth

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/config"
)

func TestMintInteractiveAPIAccessJWTFromSnapshot_explicitEmptyRoles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	uc, err := NewJWTUseCase(&config.JWTConfig{
		KeysDir:      dir,
		EdgeKeysDir:  filepath.Join(dir, "edge"),
		Issuer:       "iss",
		APIAudience:  "api-aud",
		EdgeIssuer:   "edge-iss",
		EdgeAudience: "edge-aud",
	})
	if err != nil {
		t.Fatal(err)
	}
	tok, _, _, err := uc.MintInteractiveAPIAccessJWTFromSnapshot(t.Context(), "u@x", []byte(`{"roles":[]}`), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	mc, err := uc.ParseAndValidateAPIProfileBearerToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	ra, _ := mc["roles"].([]any)
	if len(ra) != 0 {
		t.Fatalf("want empty roles, got %#v", ra)
	}
}

func TestMintInteractiveAPIAccessJWTFromSnapshot_includesProfileClaims(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	uc, err := NewJWTUseCase(&config.JWTConfig{
		KeysDir:      dir,
		EdgeKeysDir:  filepath.Join(dir, "edge"),
		Issuer:       "iss",
		APIAudience:  "api-aud",
		EdgeIssuer:   "edge-iss",
		EdgeAudience: "edge-aud",
	})
	if err != nil {
		t.Fatal(err)
	}
	snap := []byte(`{"roles":["api:member"],"email":"u@example.com","preferred_username":"u","name":"User"}`)
	tok, _, _, err := uc.MintInteractiveAPIAccessJWTFromSnapshot(t.Context(), "u@example.com", snap, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	mc, err := uc.ParseAndValidateAPIProfileBearerToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if mc["email"] != "u@example.com" {
		t.Fatalf("email=%v", mc["email"])
	}
	if mc["preferred_username"] != "u" {
		t.Fatalf("preferred_username=%v", mc["preferred_username"])
	}
	if mc["name"] != "User" {
		t.Fatalf("name=%v", mc["name"])
	}
}
