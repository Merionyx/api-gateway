//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	oapimw "github.com/oapi-codegen/fiber-middleware/v2"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/merionyx/api-gateway/internal/api-server/auth/oidc"
	"github.com/merionyx/api-gateway/internal/api-server/auth/pkce"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/container"
	httpxmw "github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/server"
	authuc "github.com/merionyx/api-gateway/internal/api-server/usecase/auth"
	sharedetcd "github.com/merionyx/api-gateway/internal/shared/etcd"
	"github.com/merionyx/api-gateway/internal/shared/grpcobs"
	"github.com/merionyx/api-gateway/internal/shared/metricshttp"
)

// Roadmap ш. 28–31: OIDC E2E against real etcd (httptest mock IdP).
// ш. 28 — login + callback; ш. 29 — refresh при 503 (degraded); ш. 30 — конкурентный refresh → 409 (CAS); ш. 31 — reuse старого our refresh после ротации → 401.

const (
	e2eOIDCProviderID = "mock-idp"
	e2eAuthCode       = "e2e-good-code"
	e2eRedirectURI    = "http://127.0.0.1:19999/callback"
	e2eOAuthClientID  = "agwctl-e2e"
	e2eClientID       = "cid"
	e2eClientSecret   = "sec"
	e2eJWKSKeyID      = "e2e-kid"
)

func rsaJWKSJSON(kid string, pub *rsa.PublicKey) []byte {
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

func newMockOIDCProvider(t *testing.T, priv *rsa.PrivateKey) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			_, _ = w.Write(rsaJWKSJSON(e2eJWKSKeyID, &priv.PublicKey))
		case r.Method == http.MethodPost && r.URL.Path == "/token":
			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if r.FormValue("client_secret") != e2eClientSecret {
				http.Error(w, "bad secret", http.StatusUnauthorized)
				return
			}
			// Callback uses authorization_code; refresh uses refresh_token (ш. 29 simulates IdP down via 503).
			if r.FormValue("grant_type") == "refresh_token" {
				http.Error(w, "idp unavailable", http.StatusServiceUnavailable)
				return
			}
			if r.FormValue("code") != e2eAuthCode {
				http.Error(w, "bad code", http.StatusBadRequest)
				return
			}
			issuer := "http://" + r.Host
			tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
				"iss":   issuer,
				"sub":   "idp-subject",
				"aud":   e2eClientID,
				"email": "e2e-user@example.com",
				"exp":   time.Now().Add(time.Hour).Unix(),
				"iat":   time.Now().Unix(),
			})
			tok.Header["kid"] = e2eJWKSKeyID
			idRaw, err := tok.SignedString(priv)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(oidc.TokenResponse{
				AccessToken:  "idp-access-token",
				TokenType:    "Bearer",
				ExpiresIn:    3600,
				RefreshToken: "idp-refresh-token",
				IDToken:      idRaw,
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func e2eAPIConfig(t *testing.T, etcdEndpoints []string, idpIssuerURL, authKeyPrefix string) *config.Config {
	t.Helper()
	kek := make([]byte, 32)
	if _, err := rand.Read(kek); err != nil {
		t.Fatal(err)
	}
	jwtDir := t.TempDir()
	return &config.Config{
		Server: config.ServerConfig{
			HTTPPort: "8080",
			GRPCPort: "19093",
			Host:     "127.0.0.1",
			CORS:     config.CORSConfig{AllowOrigins: []string{}},
		},
		Etcd: sharedetcd.EtcdConfig{
			Endpoints:   etcdEndpoints,
			DialTimeout: 5 * time.Second,
			TLS:         sharedetcd.EtcdTLSConfig{Enabled: false},
		},
		JWT: config.JWTConfig{
			KeysDir:      jwtDir,
			Issuer:       "integration-api-server",
			APIAudience:  "integration-api-http",
			EdgeIssuer:   "integration-edge",
			EdgeAudience: "integration-edge-http",
		},
		ContractSyncer: config.ContractSyncerConfig{
			Address: "127.0.0.1:1",
		},
		Readiness: config.ReadinessConfig{
			RequireContractSyncer: false,
		},
		LeaderElection: config.LeaderElectionConfig{
			Enabled: false,
		},
		GRPCRegistry:             config.GRPCRegistrySection{},
		GRPCContractSyncerClient: grpcobs.ClientTLSConfig{},
		MetricsHTTP:              metricshttp.Config{Enabled: false},
		Idempotency: config.IdempotencyConfig{
			Backend:       "memory",
			BundleSyncTTL: 24 * time.Hour,
		},
		Auth: config.AuthConfig{
			EtcdKeyPrefix:             authKeyPrefix,
			Environment:               "development",
			AllowInsecureBootstrap:    false,
			LoginIntentLeaseTTL:       10 * time.Minute,
			InteractiveAccessTokenTTL: 5 * time.Minute,
			SessionKEKBase64:          base64.StdEncoding.EncodeToString(kek),
			OIDCProviders: []config.OIDCProviderConfig{{
				ID:                   e2eOIDCProviderID,
				Name:                 "Mock IdP",
				Issuer:               idpIssuerURL,
				ClientID:             e2eClientID,
				ClientSecret:         e2eClientSecret,
				RedirectURIAllowlist: []string{e2eRedirectURI},
				ExtraScopes:          []string{"email"},
			}},
		},
	}
}

func fiberAppFromContainer(t *testing.T, c *container.Container) *fiber.App {
	t.Helper()
	app := fiber.New()
	swagger, err := apiserver.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	swagger.Servers = nil
	app.Use(recover.New())
	app.Use(oapimw.OapiRequestValidator(swagger))
	app.Use(httpxmw.APISecurity(c.JWTUseCase, c.APIKeyRepository))
	apiserver.RegisterHandlers(app, server.NewOpenAPIServer(c))
	return app
}

func TestE2E_ListOidcProviders(t *testing.T) {
	t.Parallel()

	etcdCli := NewEtcdClient(t)
	defer etcdCli.Close()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := newMockOIDCProvider(t, priv)
	t.Cleanup(idp.Close)

	authPrefix := fmt.Sprintf("/api-gateway/integration-oidc-e2e/%s", strings.ReplaceAll(uuid.NewString(), "-", ""))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = etcdCli.Delete(ctx, authPrefix, clientv3.WithPrefix())
	}()

	cfg := e2eAPIConfig(t, EtcdEndpoints(), idp.URL, authPrefix)
	cnt, err := container.NewContainer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cnt.Close()

	app := fiberAppFromContainer(t, cnt)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/v1/auth/oidc-providers", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, string(b))
	}
	var got struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Data) != 1 {
		t.Fatalf("providers: %+v", got.Data)
	}
	if got.Data[0]["id"] != e2eOIDCProviderID {
		t.Fatalf("id %v", got.Data[0]["id"])
	}
	if got.Data[0]["name"] != "Mock IdP" {
		t.Fatalf("name %v", got.Data[0]["name"])
	}
	if got.Data[0]["kind"] != "generic" {
		t.Fatalf("kind %v", got.Data[0]["kind"])
	}
}

// performOIDCAuthorizeFlow runs authorize + upstream callback + token exchange and returns access + refresh tokens.
func performOIDCAuthorizeFlow(t *testing.T, app *fiber.App) (accessToken, refreshToken string) {
	t.Helper()
	verifier, err := pkce.NewVerifier()
	if err != nil {
		t.Fatal(err)
	}
	challenge := pkce.ChallengeS256(verifier)
	clientState := "e2e-client-state"
	authorizeURL := fmt.Sprintf("http://example.com/v1/auth/authorize?provider_id=%s&redirect_uri=%s&response_type=code&client_id=%s&state=%s&code_challenge=%s&code_challenge_method=S256",
		url.QueryEscape(e2eOIDCProviderID),
		url.QueryEscape(e2eRedirectURI),
		url.QueryEscape(e2eOAuthClientID),
		url.QueryEscape(clientState),
		url.QueryEscape(challenge),
	)
	authorizeResp, err := app.Test(httptest.NewRequest(http.MethodGet, authorizeURL, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = authorizeResp.Body.Close() }()
	if authorizeResp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(authorizeResp.Body)
		t.Fatalf("authorize status %d body %s", authorizeResp.StatusCode, string(body))
	}
	loc := authorizeResp.Header.Get("Location")
	if loc == "" {
		t.Fatal("missing Location")
	}
	locU, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	upstreamState := locU.Query().Get("state")
	if upstreamState == "" {
		t.Fatalf("location %q", loc)
	}
	if !strings.Contains(locU.Path, "authorize") {
		t.Fatalf("expected authorize path in %q", loc)
	}

	cbURL := fmt.Sprintf("http://example.com/v1/auth/callback?code=%s&state=%s",
		url.QueryEscape(e2eAuthCode), url.QueryEscape(upstreamState))
	cbResp, err := app.Test(httptest.NewRequest(http.MethodGet, cbURL, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cbResp.Body.Close() }()
	if cbResp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(cbResp.Body)
		t.Fatalf("callback status %d body %s", cbResp.StatusCode, string(body))
	}
	downstreamLoc := strings.TrimSpace(cbResp.Header.Get("Location"))
	if downstreamLoc == "" {
		t.Fatal("missing downstream callback location")
	}
	downstreamU, err := url.Parse(downstreamLoc)
	if err != nil {
		t.Fatal(err)
	}
	if downstreamU.Query().Get("state") != clientState {
		t.Fatalf("downstream state mismatch: %q", downstreamU.Query().Get("state"))
	}
	localCode := downstreamU.Query().Get("code")
	if strings.TrimSpace(localCode) == "" {
		t.Fatalf("downstream location %q", downstreamLoc)
	}

	tokenForm := url.Values{}
	tokenForm.Set("grant_type", "authorization_code")
	tokenForm.Set("code", localCode)
	tokenForm.Set("redirect_uri", e2eRedirectURI)
	tokenForm.Set("client_id", e2eOAuthClientID)
	tokenForm.Set("code_verifier", verifier)
	tokenReq := httptest.NewRequest(http.MethodPost, "http://example.com/v1/auth/token", strings.NewReader(tokenForm.Encode()))
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tokenResp, err := app.Test(tokenReq)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tokenResp.Body.Close() }()
	if tokenResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(tokenResp.Body)
		t.Fatalf("token status %d body %s", tokenResp.StatusCode, string(body))
	}
	var tokens map[string]any
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokens); err != nil {
		t.Fatal(err)
	}
	at, _ := tokens["access_token"].(string)
	rt, _ := tokens["refresh_token"].(string)
	if at == "" || rt == "" {
		t.Fatalf("tokens: %+v", tokens)
	}
	return at, rt
}

func TestE2E_OIDCLoginCallback_HappyPath(t *testing.T) {
	t.Parallel()

	etcdCli := NewEtcdClient(t)
	defer etcdCli.Close()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := newMockOIDCProvider(t, priv)
	t.Cleanup(idp.Close)

	authPrefix := fmt.Sprintf("/api-gateway/integration-oidc-e2e/%s", strings.ReplaceAll(uuid.NewString(), "-", ""))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = etcdCli.Delete(ctx, authPrefix, clientv3.WithPrefix())
	}()

	cfg := e2eAPIConfig(t, EtcdEndpoints(), idp.URL, authPrefix)
	cnt, err := container.NewContainer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cnt.Close()

	app := fiberAppFromContainer(t, cnt)
	performOIDCAuthorizeFlow(t, app)
}

func TestE2E_OIDCRefresh_IdPUnavailable_Degraded(t *testing.T) {
	t.Parallel()

	etcdCli := NewEtcdClient(t)
	defer etcdCli.Close()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := newMockOIDCProvider(t, priv)
	t.Cleanup(idp.Close)

	authPrefix := fmt.Sprintf("/api-gateway/integration-oidc-e2e/%s", strings.ReplaceAll(uuid.NewString(), "-", ""))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = etcdCli.Delete(ctx, authPrefix, clientv3.WithPrefix())
	}()

	cfg := e2eAPIConfig(t, EtcdEndpoints(), idp.URL, authPrefix)
	cnt, err := container.NewContainer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cnt.Close()

	app := fiberAppFromContainer(t, cnt)
	access1, refresh1 := performOIDCAuthorizeFlow(t, app)

	refreshForm := url.Values{}
	refreshForm.Set("grant_type", "refresh_token")
	refreshForm.Set("refresh_token", refresh1)
	req := httptest.NewRequest(http.MethodPost, "http://example.com/v1/auth/token", strings.NewReader(refreshForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	refreshResp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = refreshResp.Body.Close() }()
	if refreshResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(refreshResp.Body)
		t.Fatalf("refresh status %d body %s", refreshResp.StatusCode, string(body))
	}
	var out map[string]any
	if err := json.NewDecoder(refreshResp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	access2, _ := out["access_token"].(string)
	refresh2, _ := out["refresh_token"].(string)
	if access2 == "" || refresh2 == "" {
		t.Fatalf("refresh response: %+v", out)
	}
	if access2 == access1 {
		t.Fatal("expected new access_token after degraded refresh")
	}
	if refresh2 == refresh1 {
		t.Fatal("expected rotated refresh_token after degraded refresh")
	}
}

func TestE2E_OIDCRefresh_ConcurrentSameRefreshToken_One409(t *testing.T) {
	t.Parallel()

	etcdCli := NewEtcdClient(t)
	defer etcdCli.Close()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := newMockOIDCProvider(t, priv)
	t.Cleanup(idp.Close)

	authPrefix := fmt.Sprintf("/api-gateway/integration-oidc-e2e/%s", strings.ReplaceAll(uuid.NewString(), "-", ""))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = etcdCli.Delete(ctx, authPrefix, clientv3.WithPrefix())
	}()

	cfg := e2eAPIConfig(t, EtcdEndpoints(), idp.URL, authPrefix)
	cnt, err := container.NewContainer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cnt.Close()
	if cnt.OIDCRefreshUseCase == nil {
		t.Fatal("OIDCRefreshUseCase must be configured")
	}

	app := fiberAppFromContainer(t, cnt)
	_, refresh1 := performOIDCAuthorizeFlow(t, app)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			_, err := cnt.OIDCRefreshUseCase.Refresh(ctx, authuc.OIDCRefreshRequest{RefreshToken: refresh1})
			results[idx] = err
		}(i)
	}
	close(start)
	wg.Wait()

	var nOK, nConflict int
	for _, err := range results {
		switch {
		case err == nil:
			nOK++
		case errors.Is(err, apierrors.ErrSessionRefreshConflict):
			nConflict++
		default:
			t.Fatalf("unexpected refresh error: %v", err)
		}
	}
	if nOK != 1 || nConflict != 1 {
		t.Fatalf("want exactly one success and one CAS conflict; got ok=%d conflict=%d errs=%v", nOK, nConflict, results)
	}
}

func TestE2E_OIDCRefresh_ReuseOldRefreshTokenAfterRotate_401(t *testing.T) {
	t.Parallel()

	etcdCli := NewEtcdClient(t)
	defer etcdCli.Close()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := newMockOIDCProvider(t, priv)
	t.Cleanup(idp.Close)

	authPrefix := fmt.Sprintf("/api-gateway/integration-oidc-e2e/%s", strings.ReplaceAll(uuid.NewString(), "-", ""))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, _ = etcdCli.Delete(ctx, authPrefix, clientv3.WithPrefix())
	}()

	cfg := e2eAPIConfig(t, EtcdEndpoints(), idp.URL, authPrefix)
	cnt, err := container.NewContainer(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer cnt.Close()

	app := fiberAppFromContainer(t, cnt)
	_, refresh1 := performOIDCAuthorizeFlow(t, app)

	doRefresh := func(refreshHex string) *http.Response {
		t.Helper()
		form := url.Values{}
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", refreshHex)
		req := httptest.NewRequest(http.MethodPost, "http://example.com/v1/auth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	r1 := doRefresh(refresh1)
	defer func() { _ = r1.Body.Close() }()
	if r1.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r1.Body)
		t.Fatalf("first refresh status %d body %s", r1.StatusCode, string(b))
	}
	var first map[string]any
	if err := json.NewDecoder(r1.Body).Decode(&first); err != nil {
		t.Fatal(err)
	}
	refresh2, _ := first["refresh_token"].(string)
	if refresh2 == "" || refresh2 == refresh1 {
		t.Fatalf("expected new refresh_token, got %+v", first)
	}

	rReuse := doRefresh(refresh1)
	defer func() { _ = rReuse.Body.Close() }()
	if rReuse.StatusCode != http.StatusUnauthorized {
		b, _ := io.ReadAll(rReuse.Body)
		t.Fatalf("reuse old refresh: status %d want 401 body %s", rReuse.StatusCode, string(b))
	}
	var prob map[string]any
	if err := json.NewDecoder(rReuse.Body).Decode(&prob); err != nil {
		t.Fatal(err)
	}
	if prob["code"] != "SESSION_AUTH_FAILED" {
		t.Fatalf("problem code: want SESSION_AUTH_FAILED got %v", prob["code"])
	}

	r2 := doRefresh(refresh2)
	defer func() { _ = r2.Body.Close() }()
	if r2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r2.Body)
		t.Fatalf("second refresh with new token status %d body %s", r2.StatusCode, string(b))
	}
}
