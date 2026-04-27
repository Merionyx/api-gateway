package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/merionyx/api-gateway/internal/api-server/auth/idpcache"
	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/auth/oidc"
	"github.com/merionyx/api-gateway/internal/api-server/auth/pkce"
	"github.com/merionyx/api-gateway/internal/api-server/auth/sessioncrypto"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

type memIntentRepo struct {
	mu sync.Mutex
	m  map[string]kvvalue.LoginIntentValue
}

func (m *memIntentRepo) Get(_ context.Context, id string) (kvvalue.LoginIntentValue, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.m[id]
	if !ok {
		return kvvalue.LoginIntentValue{}, apierrors.ErrNotFound
	}
	return v, nil
}

func (m *memIntentRepo) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, id)
	return nil
}

type memSessionRepo struct {
	mu     sync.Mutex
	last   kvvalue.SessionValue
	lastID string
}

func (m *memSessionRepo) Create(_ context.Context, id string, v kvvalue.SessionValue) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastID = id
	m.last = v
	return nil
}

func rsaJWKSJSONCallback(kid string, pub *rsa.PublicKey) []byte {
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(new(big.Int).SetInt64(int64(pub.E)).Bytes())
	doc := map[string]any{
		"keys": []any{
			map[string]any{"kty": "RSA", "kid": kid, "alg": "RS256", "use": "sig", "n": n, "e": e},
		},
	}
	b, _ := json.Marshal(doc)
	return b
}

func TestOIDCCallbackUseCase_Complete_HappyPath(t *testing.T) {
	t.Parallel()

	priv, gerr := rsa.GenerateKey(rand.Reader, 2048)
	if gerr != nil {
		t.Fatal(gerr)
	}
	const kid = "cb-kid"
	const clientID = "cid"
	const clientSecret = "sec"
	const redirectURI = "http://127.0.0.1:9/cb"
	const authCode = "good-code"
	pkceVerifier, verr := pkce.NewVerifier()
	if verr != nil {
		t.Fatal(verr)
	}

	intentID := uuid.NewString()
	intents := &memIntentRepo{m: map[string]kvvalue.LoginIntentValue{
		intentID: {
			ProviderID:     "p1",
			RedirectURI:    redirectURI,
			OAuthState:     intentID,
			PKCEVerifier:   pkceVerifier,
			IntentProtocol: kvvalue.DefaultIntentProtocol,
		},
	}}
	sessions := &memSessionRepo{}

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration"):
			base := "http://" + r.Host
			_ = json.NewEncoder(w).Encode(oidc.Discovery{
				Issuer:                base,
				AuthorizationEndpoint: base + "/authorize",
				TokenEndpoint:         base + "/token",
				JWKSURI:               base + "/jwks",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/jwks":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(rsaJWKSJSONCallback(kid, &priv.PublicKey))
		case r.Method == http.MethodPost && r.URL.Path == "/token":
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.FormValue("code_verifier") != pkceVerifier {
				t.Fatalf("code_verifier got %q", r.FormValue("code_verifier"))
			}
			if r.FormValue("code") != authCode {
				t.Fatalf("code got %q", r.FormValue("code"))
			}
			issuer := "http://" + r.Host
			tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
				"iss":   issuer,
				"sub":   "idp-subject",
				"aud":   clientID,
				"email": "user@example.com",
				"exp":   time.Now().Add(time.Hour).Unix(),
				"iat":   time.Now().Unix(),
			})
			tok.Header["kid"] = kid
			idRaw, err := tok.SignedString(priv)
			if err != nil {
				t.Fatal(err)
			}
			_ = json.NewEncoder(w).Encode(oidc.TokenResponse{
				AccessToken:  "idp-access",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				RefreshToken: "idp-refresh",
				IDToken:      idRaw,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	jwtUC, err := NewJWTUseCase(&config.JWTConfig{
		KeysDir:      dir,
		EdgeKeysDir:  filepath.Join(dir, "edge"),
		Issuer:       "test-issuer",
		APIAudience:  "test-aud",
		EdgeIssuer:   "edge-iss",
		EdgeAudience: "edge-aud",
	})
	if err != nil {
		t.Fatal(err)
	}
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i + 3)
	}
	kr, err := sessioncrypto.NewKeyring(sessioncrypto.KEK{ID: "k", Key: k})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kr.Close() })

	cache := idpcache.New(nil)
	uc := NewOIDCCallbackUseCase([]config.OIDCProviderConfig{{
		ID:                   "p1",
		Name:                 "Test Provider",
		Issuer:               srv.URL,
		ClientID:             clientID,
		ClientSecret:         clientSecret,
		RedirectURIAllowlist: []string{redirectURI},
	}}, intents, sessions, kr, jwtUC, srv.Client(), TokenTTLPolicy{
		DefaultAccessTTL:  5 * time.Minute,
		MaxAccessTTL:      7 * 24 * time.Hour,
		DefaultRefreshTTL: 7 * 24 * time.Hour,
		MaxRefreshTTL:     30 * 24 * time.Hour,
	}, cache, 2*time.Minute)

	out, err := uc.Complete(t.Context(), authCode, intentID)
	if err != nil {
		t.Fatal(err)
	}
	if out.AccessToken == "" || out.RefreshToken == "" {
		t.Fatalf("tokens: %+v", out)
	}
	if sessions.last.OurRefreshVerifier == "" || len(sessions.last.EncryptedIDPRefresh) == 0 {
		t.Fatalf("session: %+v", sessions.last)
	}
	if sessions.last.LoginIntentID != intentID {
		t.Fatalf("login_intent_id %q", sessions.last.LoginIntentID)
	}
	if sessions.last.RefreshExpiresAt.IsZero() {
		t.Fatal("refresh_expires_at must be set")
	}
	sessions.mu.Lock()
	sid := sessions.lastID
	sessions.mu.Unlock()
	if tok, ok := cache.Get(sid); !ok || tok != "idp-access" {
		t.Fatalf("idp access cache: ok=%v tok=%q", ok, tok)
	}
	intents.mu.Lock()
	_, ok := intents.m[intentID]
	intents.mu.Unlock()
	if ok {
		t.Fatal("intent should be deleted")
	}
}

func TestOIDCCallbackUseCase_Complete_UnknownIntent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	jwtUC, err := NewJWTUseCase(jwtTestCfg(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	k := make([]byte, 32)
	for i := range k {
		k[i] = 7
	}
	kr, err := sessioncrypto.NewKeyring(sessioncrypto.KEK{ID: "k", Key: k})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kr.Close() })
	uc := NewOIDCCallbackUseCase([]config.OIDCProviderConfig{{
		ID:                   "p1",
		Name:                 "Test Provider",
		Issuer:               "https://unused.example",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://x"},
	}}, &memIntentRepo{m: map[string]kvvalue.LoginIntentValue{}}, &memSessionRepo{}, kr, jwtUC, http.DefaultClient, TokenTTLPolicy{
		DefaultAccessTTL:  time.Minute,
		MaxAccessTTL:      7 * 24 * time.Hour,
		DefaultRefreshTTL: 7 * 24 * time.Hour,
		MaxRefreshTTL:     30 * 24 * time.Hour,
	}, nil, 0)
	_, err = uc.Complete(t.Context(), "code", uuid.NewString())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, apierrors.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestMapCallbackError_MissingIDTokenDetail(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf("%w: %w", oidc.ErrTokenExchange, oidc.ErrMissingIDTokenInTokenResponse)
	st, code, detail := MapCallbackError(wrapped)
	if st != 401 || code != "OIDC_TOKEN_EXCHANGE_FAILED" {
		t.Fatalf("got status=%d code=%q", st, code)
	}
	if !strings.Contains(detail, "id_token") || !strings.Contains(detail, "OAuth Apps") {
		t.Fatalf("unexpected detail: %s", detail)
	}
}

func TestOIDCCallbackUseCase_Complete_GitHubFallbackWithoutIDToken(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	jwtUC, err := NewJWTUseCase(jwtTestCfg(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	k := make([]byte, 32)
	for i := range k {
		k[i] = 7
	}
	kr, err := sessioncrypto.NewKeyring(sessioncrypto.KEK{ID: "k", Key: k})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kr.Close() })

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                 srv.URL,
				"authorization_endpoint": srv.URL + "/authorize",
				"token_endpoint":         srv.URL + "/token",
				"jwks_uri":               srv.URL + "/jwks",
			})
		case r.URL.Path == "/token" && r.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "gho_live_like",
				"token_type":   "bearer",
				"scope":        "read:org",
			})
		case r.URL.Path == "/user":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":    42,
				"login": "octocat",
				"name":  "Mona Octocat",
				"email": "octo@example.com",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	oldBase := githubRESTBaseURL
	githubRESTBaseURL = srv.URL
	t.Cleanup(func() { githubRESTBaseURL = oldBase })

	intentID := uuid.NewString()
	intents := &memIntentRepo{m: map[string]kvvalue.LoginIntentValue{
		intentID: {
			ProviderID:   "github",
			RedirectURI:  "http://127.0.0.1:21987/callback",
			OAuthState:   intentID,
			PKCEVerifier: "pkce",
		},
	}}
	sessions := &memSessionRepo{}

	uc := NewOIDCCallbackUseCase([]config.OIDCProviderConfig{{
		ID:                   "github",
		Name:                 "GitHub",
		Kind:                 "github",
		Issuer:               srv.URL,
		ClientID:             "cid",
		ClientSecret:         "sec",
		RedirectURIAllowlist: []string{"http://127.0.0.1:21987/callback"},
		GitHub:               &config.GitHubOIDCProviderConfig{},
	}}, intents, sessions, kr, jwtUC, srv.Client(), TokenTTLPolicy{
		DefaultAccessTTL:  time.Minute,
		MaxAccessTTL:      7 * 24 * time.Hour,
		DefaultRefreshTTL: 7 * 24 * time.Hour,
		MaxRefreshTTL:     30 * 24 * time.Hour,
	}, nil, 0)

	out, err := uc.Complete(t.Context(), "authcode", intentID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.AccessToken) == "" || strings.TrimSpace(out.RefreshToken) == "" {
		t.Fatalf("unexpected output: %+v", out)
	}
	var snap map[string]any
	if err := json.Unmarshal(sessions.last.ClaimsSnapshot, &snap); err != nil {
		t.Fatal(err)
	}
	if got, _ := snap["preferred_username"].(string); got != "octocat" {
		t.Fatalf("preferred_username=%v", snap["preferred_username"])
	}
	if got, _ := snap["sub"].(string); got != "github-id:42" {
		t.Fatalf("sub=%v", snap["sub"])
	}
}
