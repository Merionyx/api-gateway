package auth

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/config"
)

func TestParseAndValidateAPIProfileBearerToken_acceptsInteractiveMint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	uc, err := NewJWTUseCase(jwtTestCfg(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	tok, _, _, err := uc.MintInteractiveAPIAccessJWT(t.Context(), "user@example.com", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	mc, err := uc.ParseAndValidateAPIProfileBearerToken(tok)
	if err != nil {
		t.Fatal(err)
	}
	if mc["sub"] != "user@example.com" {
		t.Fatalf("sub %v", mc["sub"])
	}
}

func TestParseAndValidateAPIProfileBearerToken_rejectsEdgeToken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	uc, err := NewJWTUseCase(&config.JWTConfig{
		KeysDir:      dir,
		EdgeKeysDir:  filepath.Join(dir, "edge"),
		Issuer:       "api-iss",
		APIAudience:  "api-aud",
		EdgeIssuer:   "edge-iss",
		EdgeAudience: "edge-aud",
	})
	if err != nil {
		t.Fatal(err)
	}
	edgeTok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"iss": "edge-iss",
		"aud": "edge-aud",
		"sub": "app",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	kp := uc.edgeSigningKeys[uc.edgeActiveKeyID]
	if kp == nil {
		t.Fatal("no edge key")
	}
	edgeTok.Header["kid"] = kp.Kid
	s, err := edgeTok.SignedString(kp.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	_, err = uc.ParseAndValidateAPIProfileBearerToken(s)
	if err == nil {
		t.Fatal("expected error for Edge-profile token")
	}
}
