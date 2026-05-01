package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func rsaJWKSJSON(kid string, pub *rsa.PublicKey) []byte {
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(new(big.Int).SetInt64(int64(pub.E)).Bytes())
	doc := map[string]any{
		"keys": []any{
			map[string]any{
				"kty": "RSA",
				"kid": kid,
				"alg": "RS256",
				"use": "sig",
				"n":   n,
				"e":   e,
			},
		},
	}
	b, _ := json.Marshal(doc)
	return b
}

func TestDiscovery_TokenExchange_ValidateIDToken(t *testing.T) {
	t.Parallel()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "unit-test-kid"
	const clientID = "test-client-id"
	const clientSecret = "test-client-secret"
	const redirectURI = "http://localhost/cb"
	const authCode = "auth-code-xyz"
	const nonce = "nonce-abc"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/.well-known/openid-configuration":
			issuer := "http://" + r.Host
			_ = json.NewEncoder(w).Encode(Discovery{
				Issuer:                issuer,
				AuthorizationEndpoint: issuer + "/authorize",
				TokenEndpoint:         issuer + "/token",
				JWKSURI:               issuer + "/jwks",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/jwks":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(rsaJWKSJSON(kid, &priv.PublicKey))
		case r.Method == http.MethodPost && r.URL.Path == "/token":
			issuer := "http://" + r.Host
			tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
				"iss":   issuer,
				"sub":   "subject-1",
				"aud":   clientID,
				"exp":   time.Now().Add(time.Hour).Unix(),
				"iat":   time.Now().Unix(),
				"nonce": nonce,
			})
			tok.Header["kid"] = kid
			idRaw, err := tok.SignedString(priv)
			if err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(TokenResponse{
				AccessToken: "at-mock",
				TokenType:   "Bearer",
				ExpiresIn:   3600,
				IDToken:     idRaw,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	hc := srv.Client()
	base := NormalizeIssuer(srv.URL)

	disc, err := FetchDiscovery(t.Context(), hc, base, true)
	if err != nil {
		t.Fatal(err)
	}
	if disc.TokenEndpoint == "" || disc.JWKSURI == "" {
		t.Fatalf("discovery: %+v", disc)
	}

	tr, err := ExchangeAuthorizationCode(t.Context(), hc, disc.TokenEndpoint, clientID, clientSecret, authCode, redirectURI, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if tr.AccessToken != "at-mock" || tr.IDToken == "" {
		t.Fatalf("token response: %+v", tr)
	}

	mc, err := ValidateIDToken(t.Context(), hc, disc, tr.IDToken, ValidateIDTokenOptions{
		ExpectedIssuer:   disc.Issuer,
		ExpectedAudience: clientID,
		ExpectedNonce:    nonce,
	})
	if err != nil {
		t.Fatal(err)
	}
	if mc["sub"] != "subject-1" {
		t.Fatalf("claims: %v", mc)
	}
}

func TestExchangeAuthorizationCode_MissingIDToken(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(TokenResponse{
			AccessToken: "ghu_only",
			TokenType:   "bearer",
		})
	}))
	t.Cleanup(srv.Close)
	got, err := ExchangeAuthorizationCode(context.Background(), srv.Client(), srv.URL+"/token", "cid", "sec", "code", "http://cb", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.AccessToken != "ghu_only" || got.IDToken != "" {
		t.Fatalf("unexpected token response: %+v", got)
	}
}

func TestExchangeAuthorizationCode_OAuth2ErrorJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "redirect_uri_mismatch",
			"error_description": "The redirect_uri MUST match the registered callback URL for this application.",
		})
	}))
	t.Cleanup(srv.Close)
	_, err := ExchangeAuthorizationCode(context.Background(), srv.Client(), srv.URL+"/token", "cid", "sec", "code", "http://cb", "", true)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrTokenExchange) {
		t.Fatalf("want ErrTokenExchange, got %v", err)
	}
	var te *OAuth2TokenError
	if !errors.As(err, &te) || te.Code != "redirect_uri_mismatch" {
		t.Fatalf("want OAuth2TokenError, got %v", err)
	}
}

func TestExchangeAuthorizationCode_SendsPKCEVerifier(t *testing.T) {
	t.Parallel()
	var gotVerifier string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/token" {
			http.NotFound(w, r)
			return
		}
		_ = r.ParseForm()
		gotVerifier = r.FormValue("code_verifier")
		_ = json.NewEncoder(w).Encode(TokenResponse{
			AccessToken: "x",
			TokenType:   "Bearer",
			IDToken:     "not-valid-for-this-test",
		})
	}))
	t.Cleanup(srv.Close)
	_, err := ExchangeAuthorizationCode(context.Background(), srv.Client(), srv.URL+"/token", "cid", "sec", "code", "http://cb", "verifier-xyz", true)
	if err != nil {
		t.Fatal(err)
	}
	if gotVerifier != "verifier-xyz" {
		t.Fatalf("code_verifier: got %q", gotVerifier)
	}
}

func TestExchangeAuthorizationCode_RejectsHTTPByDefault(t *testing.T) {
	t.Parallel()
	_, err := ExchangeAuthorizationCode(context.Background(), http.DefaultClient, "http://idp.example/token", "cid", "sec", "code", "http://cb", "", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrTokenExchange) || !errors.Is(err, ErrInsecureEndpoint) {
		t.Fatalf("got %v", err)
	}
}

func TestFetchDiscovery_BadStatus(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	_, err := FetchDiscovery(context.Background(), srv.Client(), srv.URL, true)
	if err == nil || !errors.Is(err, ErrDiscovery) {
		t.Fatalf("got %v", err)
	}
}
