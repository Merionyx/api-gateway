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

// OIDCLoginUseCase handles GET /api/v1/auth/login (roadmap ш. 13).
type OIDCLoginUseCase struct {
	byID     map[string]config.OIDCProviderConfig
	leaseTTL time.Duration
	intents  loginIntentStore
	hc       *http.Client
}

// NewOIDCLoginUseCase builds a use case. leaseTTL<=0 defaults to 15m; hc nil uses http.DefaultClient.
func NewOIDCLoginUseCase(
	providers []config.OIDCProviderConfig,
	leaseTTL time.Duration,
	intents loginIntentStore,
	hc *http.Client,
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
	return &OIDCLoginUseCase{byID: by, leaseTTL: leaseTTL, intents: intents, hc: hc}
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
func (u *OIDCLoginUseCase) Start(ctx context.Context, providerID, redirectURI, nonce string) (string, error) {
	if len(u.byID) == 0 {
		return "", apierrors.ErrOIDCNotConfigured
	}
	p, ok := u.byID[strings.TrimSpace(providerID)]
	if !ok {
		return "", apierrors.ErrOIDCUnknownProvider
	}
	red := strings.TrimSpace(redirectURI)
	if _, err := url.ParseRequestURI(red); err != nil {
		return "", fmt.Errorf("%w: redirect_uri: %w", apierrors.ErrInvalidInput, err)
	}
	if !RedirectURIAllowlisted(p.RedirectURIAllowlist, redirectURI) {
		return "", apierrors.ErrOIDCRedirectNotAllowlisted
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
		ProviderID:   strings.TrimSpace(p.ID),
		RedirectURI:  red,
		OAuthState:   state,
		PKCEVerifier: ver,
		Nonce:        strings.TrimSpace(nonce),
	}
	if err := u.intents.Create(ctx, intentID, val, u.leaseTTL); err != nil {
		return "", err
	}

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", val.RedirectURI)
	q.Set("scope", buildOIDCScope(p))
	q.Set("state", state)
	q.Set("code_challenge", chal)
	q.Set("code_challenge_method", "S256")
	if val.Nonce != "" {
		q.Set("nonce", val.Nonce)
	}

	authURL, err := mergeAuthorizeQuery(disc.AuthorizationEndpoint, q)
	if err != nil {
		return "", err
	}
	return authURL, nil
}

func buildOIDCScope(p config.OIDCProviderConfig) string {
	parts := []string{"openid"}
	for _, s := range p.ExtraScopes {
		s = strings.TrimSpace(s)
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " ")
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

// MapStartError classifies errors from Start for HTTP mapping (returns same err if unclassified).
func MapStartError(err error) (int, string, string) {
	switch {
	case err == nil:
		return 0, "", ""
	case errors.Is(err, apierrors.ErrOIDCNotConfigured):
		return 400, "OIDC_NOT_CONFIGURED", "Configure auth.oidc_providers to enable browser login."
	case errors.Is(err, apierrors.ErrOIDCUnknownProvider):
		return 400, "OIDC_UNKNOWN_PROVIDER", "Unknown provider_id."
	case errors.Is(err, apierrors.ErrOIDCRedirectNotAllowlisted):
		return 400, "OIDC_REDIRECT_NOT_ALLOWED", "redirect_uri is not allowlisted for this provider."
	case errors.Is(err, apierrors.ErrInvalidInput):
		return 400, "INVALID_REDIRECT_URI", "redirect_uri is not a valid absolute URI."
	case errors.Is(err, oidc.ErrDiscovery):
		return 502, "OIDC_DISCOVERY_FAILED", "Could not load OpenID configuration from the issuer."
	case errors.Is(err, apierrors.ErrStoreAccess):
		return 503, "STORE_UNAVAILABLE", "Could not persist login context."
	default:
		return 500, "INTERNAL_ERROR", "Login could not be started."
	}
}
