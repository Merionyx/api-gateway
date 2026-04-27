package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/auth/oidc"
	"github.com/merionyx/api-gateway/internal/api-server/auth/pkce"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

const defaultIntentLease = 15 * time.Minute

// loginIntentStore persists short-lived OIDC login context (e.g. etcd LoginIntentRepository).
type loginIntentStore interface {
	Create(ctx context.Context, intentID string, v kvvalue.LoginIntentValue, leaseTTL time.Duration) error
}

type OIDCLoginStartRequest struct {
	ProviderID          string
	RedirectURI         string
	ServerCallbackURI   string
	Nonce               string
	RequestedAccessTTL  time.Duration
	RequestedRefreshTTL time.Duration
	ResponseType        string
	ClientID            string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
}

// OIDCLoginUseCase handles GET /v1/auth/authorize (roadmap ш. 13).
type OIDCLoginUseCase struct {
	byID           map[string]config.OIDCProviderConfig
	leaseTTL       time.Duration
	intents        loginIntentStore
	hc             *http.Client
	tokenTTLPolicy TokenTTLPolicy
}

// NewOIDCLoginUseCase builds a use case. leaseTTL<=0 defaults to 15m; hc nil uses http.DefaultClient.
func NewOIDCLoginUseCase(
	providers []config.OIDCProviderConfig,
	leaseTTL time.Duration,
	intents loginIntentStore,
	hc *http.Client,
	tokenTTLPolicy TokenTTLPolicy,
) *OIDCLoginUseCase {
	by := make(map[string]config.OIDCProviderConfig, len(providers))
	for _, p := range providers {
		by[strings.TrimSpace(p.ID)] = p
	}
	if leaseTTL <= 0 {
		leaseTTL = defaultIntentLease
	}
	if hc == nil {
		hc = http.DefaultClient
	}
	return &OIDCLoginUseCase{byID: by, leaseTTL: leaseTTL, intents: intents, hc: hc, tokenTTLPolicy: tokenTTLPolicy}
}

// RedirectURIAllowlisted reports whether redirect matches one allowlisted entry (exact string match after TrimSpace).
func RedirectURIAllowlisted(allow []string, redirect string) bool {
	r := strings.TrimSpace(redirect)
	for _, a := range allow {
		if strings.TrimSpace(a) == r {
			return true
		}
	}
	return false
}

// Start persists a login-intent and returns the IdP authorization URL for HTTP 302 Location.
func (u *OIDCLoginUseCase) Start(ctx context.Context, req OIDCLoginStartRequest) (string, error) {
	if len(u.byID) == 0 {
		return "", apierrors.ErrOIDCNotConfigured
	}
	if err := validateOAuthAuthorizeRequest(req); err != nil {
		return "", fmt.Errorf("%w: %s", apierrors.ErrInvalidInput, err.Error())
	}
	resolvedTTLs, err := resolveRequestedTokenTTLs(u.tokenTTLPolicy, RequestedTokenTTLs{
		AccessTTL:  req.RequestedAccessTTL,
		RefreshTTL: req.RequestedRefreshTTL,
	})
	if err != nil {
		return "", fmt.Errorf("%w: %s", apierrors.ErrInvalidInput, err.Error())
	}

	providerID, err := u.resolveProviderID(strings.TrimSpace(req.ProviderID))
	if err != nil {
		return "", err
	}
	p, ok := u.byID[providerID]
	if !ok {
		return "", apierrors.ErrOIDCUnknownProvider
	}

	clientRedirectURI := strings.TrimSpace(req.RedirectURI)
	if _, err := url.ParseRequestURI(clientRedirectURI); err != nil {
		return "", fmt.Errorf("%w: redirect_uri: %w", apierrors.ErrInvalidInput, err)
	}
	if !RedirectURIAllowlisted(p.RedirectURIAllowlist, req.RedirectURI) {
		return "", apierrors.ErrOIDCRedirectNotAllowlisted
	}

	idpRedirectURI := strings.TrimSpace(req.ServerCallbackURI)
	if _, err := url.ParseRequestURI(idpRedirectURI); err != nil {
		return "", fmt.Errorf("%w: server_callback_uri: %w", apierrors.ErrInvalidInput, err)
	}

	issuer := oidc.NormalizeIssuer(p.Issuer)
	disc, err := oidc.FetchDiscovery(ctx, u.hc, issuer)
	if err != nil {
		return "", fmt.Errorf("%w: %w", oidc.ErrDiscovery, err)
	}
	if strings.TrimSpace(disc.AuthorizationEndpoint) == "" {
		return "", fmt.Errorf("%w: missing authorization_endpoint", oidc.ErrDiscovery)
	}

	ver, err := pkce.NewVerifier()
	if err != nil {
		return "", err
	}
	chal := pkce.ChallengeS256(ver)

	// state doubles as opaque login-intent id so callback can load login-intents/{state} (ш. 14).
	intentID := uuid.NewString()
	state := intentID

	val := kvvalue.LoginIntentValue{
		ProviderID:                      strings.TrimSpace(p.ID),
		RedirectURI:                     idpRedirectURI,
		OAuthState:                      state,
		PKCEVerifier:                    ver,
		Nonce:                           strings.TrimSpace(req.Nonce),
		RequestedAccessTokenTTLSeconds:  int64(resolvedTTLs.AccessTTL / time.Second),
		RequestedRefreshTokenTTLSeconds: int64(resolvedTTLs.RefreshTTL / time.Second),
		OAuthClientID:                   strings.TrimSpace(req.ClientID),
		OAuthClientRedirectURI:          clientRedirectURI,
		OAuthClientState:                strings.TrimSpace(req.State),
		OAuthClientCodeChallenge:        strings.TrimSpace(req.CodeChallenge),
		OAuthClientCodeChallengeMethod:  "S256",
	}
	if err := u.intents.Create(ctx, intentID, val, u.leaseTTL); err != nil {
		return "", err
	}

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", val.RedirectURI)
	if scope := buildOIDCScope(p); strings.TrimSpace(scope) != "" {
		q.Set("scope", scope)
	}
	q.Set("state", state)
	q.Set("code_challenge", chal)
	q.Set("code_challenge_method", "S256")
	if val.Nonce != "" {
		q.Set("nonce", val.Nonce)
	}
	applyProviderAuthorizeParams(q, p)

	authURL, err := mergeAuthorizeQuery(disc.AuthorizationEndpoint, q)
	if err != nil {
		return "", err
	}
	return authURL, nil
}

func buildOIDCScope(p config.OIDCProviderConfig) string {
	if p.IsGitHubOIDCProvider() && p.GitHubAuthFlow() == "github_app" {
		return ""
	}
	parts := []string{"openid"}
	extra := append([]string(nil), p.ExtraScopes...)
	if p.IsGitHubOIDCProvider() && p.GitHubAuthFlow() == "oauth_app" && !scopeInListCI(extra, "read:org") {
		extra = append(extra, "read:org")
	}
	if p.IsGitLabOIDCProvider() {
		if !scopeInListCI(extra, "read_api") {
			extra = append(extra, "read_api")
		}
		// Match typical GitLab OIDC userinfo: id_token often omits email without these scopes.
		if !scopeInListCI(extra, "email") {
			extra = append(extra, "email")
		}
		if !scopeInListCI(extra, "profile") {
			extra = append(extra, "profile")
		}
	}
	if p.IsGoogleOIDCProvider() {
		if !scopeInListCI(extra, "email") {
			extra = append(extra, "email")
		}
		if !scopeInListCI(extra, "profile") {
			extra = append(extra, "profile")
		}
	}
	if p.IsOktaOIDCProvider() && !scopeInListCI(extra, "groups") {
		extra = append(extra, "groups")
	}
	if p.IsEntraOIDCProvider() {
		if !scopeInListCI(extra, "email") {
			extra = append(extra, "email")
		}
		if !scopeInListCI(extra, "profile") {
			extra = append(extra, "profile")
		}
	}
	for _, s := range extra {
		s = strings.TrimSpace(s)
		if s == "" || scopeInListCI(parts, s) {
			continue
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, " ")
}

func scopeInListCI(list []string, s string) bool {
	want := strings.TrimSpace(strings.ToLower(s))
	for _, x := range list {
		if strings.TrimSpace(strings.ToLower(x)) == want {
			return true
		}
	}
	return false
}

func applyProviderAuthorizeParams(q url.Values, p config.OIDCProviderConfig) {
	if q == nil {
		return
	}
	if p.IsGoogleOIDCProvider() {
		// Google returns refresh_token for web-server OAuth when offline access is requested.
		// prompt=consent forces a fresh consent screen so re-logins can recover a missing refresh token.
		q.Set("access_type", "offline")
		q.Set("include_granted_scopes", "true")
		q.Set("prompt", "consent")
	}
}

func mergeAuthorizeQuery(authEndpoint string, add url.Values) (string, error) {
	u, err := url.Parse(authEndpoint)
	if err != nil {
		return "", fmt.Errorf("authorize url: %w", err)
	}
	existing := u.Query()
	for k, vs := range add {
		for _, v := range vs {
			existing.Add(k, v)
		}
	}
	u.RawQuery = existing.Encode()
	return u.String(), nil
}

func (u *OIDCLoginUseCase) resolveProviderID(providerID string) (string, error) {
	if providerID != "" {
		if _, ok := u.byID[providerID]; !ok {
			return "", apierrors.ErrOIDCUnknownProvider
		}
		return providerID, nil
	}
	if len(u.byID) == 1 {
		for id := range u.byID {
			return id, nil
		}
	}
	return "", apierrors.ErrOIDCUnknownProvider
}

func validateOAuthAuthorizeRequest(req OIDCLoginStartRequest) error {
	if !strings.EqualFold(strings.TrimSpace(req.ResponseType), "code") {
		return errors.New("response_type must be code")
	}
	if strings.TrimSpace(req.ClientID) == "" {
		return errors.New("client_id is required")
	}
	if strings.TrimSpace(req.CodeChallenge) == "" {
		return errors.New("code_challenge is required")
	}
	if !strings.EqualFold(strings.TrimSpace(req.CodeChallengeMethod), "S256") {
		return errors.New("code_challenge_method must be S256")
	}
	cc := strings.TrimSpace(req.CodeChallenge)
	if len(cc) < 43 || len(cc) > 128 {
		return errors.New("code_challenge length must be in [43,128]")
	}
	for i := 0; i < len(cc); i++ {
		c := cc[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '-', c == '.', c == '_', c == '~':
		default:
			return errors.New("code_challenge has unsupported characters")
		}
	}
	return nil
}

// MapStartError classifies errors from Start for HTTP mapping (returns same err if unclassified).
func MapStartError(err error) (int, string, string) {
	switch {
	case err == nil:
		return 0, "", ""
	case errors.Is(err, apierrors.ErrOIDCNotConfigured):
		return 400, "OIDC_NOT_CONFIGURED", "Configure auth.oidc_providers to enable browser login."
	case errors.Is(err, apierrors.ErrOIDCUnknownProvider):
		return 400, "OIDC_UNKNOWN_PROVIDER", "Unknown provider_id or ambiguous provider selection."
	case errors.Is(err, apierrors.ErrOIDCRedirectNotAllowlisted):
		return 400, "OIDC_REDIRECT_NOT_ALLOWED", "redirect_uri is not allowlisted for this provider."
	case errors.Is(err, apierrors.ErrInvalidInput):
		return 400, "INVALID_AUTHORIZE_REQUEST", err.Error()
	case errors.Is(err, oidc.ErrDiscovery):
		return 502, "OIDC_DISCOVERY_FAILED", "Could not load OpenID configuration from the issuer."
	case errors.Is(err, apierrors.ErrStoreAccess):
		return 503, "STORE_UNAVAILABLE", "Could not persist login context."
	default:
		return 500, "INTERNAL_ERROR", "Login could not be started."
	}
}
