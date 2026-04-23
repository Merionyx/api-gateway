//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	oapimw "github.com/oapi-codegen/fiber-middleware/v2"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/merionyx/api-gateway/internal/api-server/auth/oidc"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/container"
	httpxmw "github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	"github.com/merionyx/api-gateway/internal/api-server/server"
	sharedetcd "github.com/merionyx/api-gateway/internal/shared/etcd"
	"github.com/merionyx/api-gateway/internal/shared/grpcobs"
	"github.com/merionyx/api-gateway/internal/shared/metricshttp"
)

// Roadmap ш. 28–29: OIDC E2E against real etcd (httptest mock IdP).
// ш. 28 — login + callback; ш. 29 — POST refresh when IdP token endpoint returns 5xx (degraded refresh, roadmap ш. 18).

const (
	e2eOIDCProviderID = "mock-idp"
	e2eAuthCode       = "e2e-good-code"
	e2eRedirectURI    = "http://127.0.0.1:19999/callback"
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

// performOIDCLoginCallback runs GET login → GET callback and returns access + refresh tokens (roadmap ш. 28).
func performOIDCLoginCallback(t *testing.T, app *fiber.App) (accessToken, refreshToken string) {
	t.Helper()
	loginURL := fmt.Sprintf("http://example.com/api/v1/auth/login?provider_id=%s&redirect_uri=%s",
		url.QueryEscape(e2eOIDCProviderID), url.QueryEscape(e2eRedirectURI))
	loginResp, err := app.Test(httptest.NewRequest(http.MethodGet, loginURL, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = loginResp.Body.Close() }()
	if loginResp.StatusCode != http.StatusFound {
		body, _ := io.ReadAll(loginResp.Body)
		t.Fatalf("login status %d body %s", loginResp.StatusCode, string(body))
	}
	loc := loginResp.Header.Get("Location")
	if loc == "" {
		t.Fatal("missing Location")
	}
	locU, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	state := locU.Query().Get("state")
	if state == "" {
		t.Fatalf("location %q", loc)
	}
	if !strings.Contains(locU.Path, "authorize") {
		t.Fatalf("expected authorize path in %q", loc)
	}

	cbURL := fmt.Sprintf("http://example.com/api/v1/auth/callback?code=%s&state=%s",
		url.QueryEscape(e2eAuthCode), url.QueryEscape(state))
	cbResp, err := app.Test(httptest.NewRequest(http.MethodGet, cbURL, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cbResp.Body.Close() }()
	if cbResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(cbResp.Body)
		t.Fatalf("callback status %d body %s", cbResp.StatusCode, string(body))
	}
	var tokens map[string]any
	if err := json.NewDecoder(cbResp.Body).Decode(&tokens); err != nil {
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
	performOIDCLoginCallback(t, app)
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
	access1, refresh1 := performOIDCLoginCallback(t, app)

	refreshBody := fmt.Sprintf(`{"refresh_token":%q}`, refresh1)
	req := httptest.NewRequest(http.MethodPost, "http://example.com/api/v1/auth/refresh", bytes.NewReader([]byte(refreshBody)))
	req.Header.Set("Content-Type", "application/json")
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
