package jwt

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAudienceMatchesExpected(t *testing.T) {
	t.Parallel()
	want := "edge-aud"
	if !audienceMatchesExpected(jwt.MapClaims{"aud": want}, want) {
		t.Fatal("string aud")
	}
	if !audienceMatchesExpected(jwt.MapClaims{"aud": []any{want, "other"}}, want) {
		t.Fatal("slice any aud")
	}
	if !audienceMatchesExpected(jwt.MapClaims{"aud": []string{want}}, want) {
		t.Fatal("slice string aud")
	}
	if audienceMatchesExpected(jwt.MapClaims{"aud": "wrong"}, want) {
		t.Fatal("wrong string")
	}
	if audienceMatchesExpected(jwt.MapClaims{}, want) {
		t.Fatal("missing aud")
	}
}

func TestJWTValidator_ValidateToken_edgeProfile_ok(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "k1"
	jwks := map[string]any{
		"keys": []any{
			map[string]any{
				"kty": "OKP", "crv": "Ed25519", "kid": kid, "alg": "EdDSA", "use": "sig",
				"x": base64.RawURLEncoding.EncodeToString(pub),
			},
		},
	}
	jwksBody, _ := json.Marshal(jwks)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(jwksBody)
	}))
	defer srv.Close()

	v := NewJWTValidator(srv.URL, "edge-iss", "edge-aud")

	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"iss": "edge-iss",
		"aud": "edge-aud",
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.ValidateToken(signed)
	if err != nil {
		t.Fatal(err)
	}
}

func TestJWTValidator_ValidateToken_edgeProfile_wrongIss(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "k1"
	jwksBody := []byte(fmt.Sprintf(`{"keys":[{"kty":"OKP","crv":"Ed25519","kid":%q,"alg":"EdDSA","use":"sig","x":%q}]}`,
		kid, base64.RawURLEncoding.EncodeToString(pub)))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(jwksBody)
	}))
	defer srv.Close()

	v := NewJWTValidator(srv.URL, "edge-iss", "edge-aud")
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"iss": "api-gateway-api-server",
		"aud": "edge-aud",
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.ValidateToken(signed)
	if err == nil {
		t.Fatal("expected iss mismatch error")
	}
}

func TestJWTValidator_ValidateToken_edgeProfile_wrongAud(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "k1"
	jwksBody := []byte(fmt.Sprintf(`{"keys":[{"kty":"OKP","crv":"Ed25519","kid":%q,"alg":"EdDSA","use":"sig","x":%q}]}`,
		kid, base64.RawURLEncoding.EncodeToString(pub)))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(jwksBody)
	}))
	defer srv.Close()

	v := NewJWTValidator(srv.URL, "edge-iss", "edge-aud")
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"iss": "edge-iss",
		"aud": "api-gateway-api-http",
		"iat": now.Unix(),
		"exp": now.Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.ValidateToken(signed)
	if err == nil {
		t.Fatal("expected aud mismatch error")
	}
}
