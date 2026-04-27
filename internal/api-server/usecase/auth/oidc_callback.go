package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/auth/idpcache"
	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/auth/oidc"
	"github.com/merionyx/api-gateway/internal/api-server/auth/sessioncrypto"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

// loginIntentReadDeleter loads and removes OIDC login-intent rows (etcd implementation).
type loginIntentReadDeleter interface {
	Get(ctx context.Context, intentID string) (kvvalue.LoginIntentValue, error)
	Delete(ctx context.Context, intentID string) error
}

// sessionCreator persists new interactive sessions (etcd SessionRepository).
type sessionCreator interface {
	Create(ctx context.Context, sessionID string, v kvvalue.SessionValue) error
}

// envelopeSealer encrypts IdP refresh material for SessionValue (sessioncrypto.Keyring).
type envelopeSealer interface {
	Seal(plaintext []byte) (sessioncrypto.Envelope, error)
}

// OIDCCallbackUseCase completes GET /v1/auth/callback (roadmap ш. 14).
type OIDCCallbackUseCase struct {
	byID            map[string]config.OIDCProviderConfig
	intents         loginIntentReadDeleter
	sessions        sessionCreator
	sealer          envelopeSealer
	jwt             *JWTUseCase
	hc              *http.Client
	tokenTTLPolicy  TokenTTLPolicy
	idpCache        *idpcache.Cache
	idpOpaqueMaxTTL time.Duration
}

type OIDCCallbackResult struct {
	RedirectURL     string
	AuthorizationID string
}

// NewOIDCCallbackUseCase wires callback dependencies. hc nil uses http.DefaultClient.
func NewOIDCCallbackUseCase(
	providers []config.OIDCProviderConfig,
	intents loginIntentReadDeleter,
	sessions sessionCreator,
	sealer envelopeSealer,
	jwtUC *JWTUseCase,
	hc *http.Client,
	tokenTTLPolicy TokenTTLPolicy,
	idpCache *idpcache.Cache,
	idpOpaqueMaxTTL time.Duration,
) *OIDCCallbackUseCase {
	by := make(map[string]config.OIDCProviderConfig, len(providers))
	for _, p := range providers {
		by[strings.TrimSpace(p.ID)] = p
	}
	if hc == nil {
		hc = http.DefaultClient
	}
	return &OIDCCallbackUseCase{
		byID:            by,
		intents:         intents,
		sessions:        sessions,
		sealer:          sealer,
		jwt:             jwtUC,
		hc:              hc,
		tokenTTLPolicy:  tokenTTLPolicy,
		idpCache:        idpCache,
		idpOpaqueMaxTTL: idpOpaqueMaxTTL,
	}
}

// CompleteWithResult exchanges provider code and returns OAuth redirect data for the downstream client.
func (u *OIDCCallbackUseCase) CompleteWithResult(ctx context.Context, code, state string) (OIDCCallbackResult, error) {
	var out OIDCCallbackResult
	if len(u.byID) == 0 {
		return out, apierrors.ErrOIDCNotConfigured
	}
	code = strings.TrimSpace(code)
	state = strings.TrimSpace(state)
	if code == "" || state == "" {
		return out, fmt.Errorf("%w: code and state are required", apierrors.ErrInvalidInput)
	}

	intent, err := u.intents.Get(ctx, state)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return out, fmt.Errorf("%w: login intent", apierrors.ErrNotFound)
		}
		return out, err
	}
	if intent.OAuthState != state {
		return out, fmt.Errorf("%w: oauth state mismatch", apierrors.ErrInvalidInput)
	}
	if !isOAuthClientIntent(intent) {
		return out, fmt.Errorf("%w: oauth callback intent is invalid", apierrors.ErrInvalidInput)
	}

	p, ok := u.byID[strings.TrimSpace(intent.ProviderID)]
	if !ok {
		return out, apierrors.ErrOIDCUnknownProvider
	}

	issuer := oidc.NormalizeIssuer(p.Issuer)
	disc, err := oidc.FetchDiscovery(ctx, u.hc, issuer)
	if err != nil {
		return out, fmt.Errorf("%w: %w", oidc.ErrDiscovery, err)
	}

	tr, err := oidc.ExchangeAuthorizationCode(ctx, u.hc, disc.TokenEndpoint, p.ClientID, p.ClientSecret, code, intent.RedirectURI, intent.PKCEVerifier)
	if err != nil {
		return out, err
	}

	var idClaims jwt.MapClaims
	if strings.TrimSpace(tr.IDToken) == "" {
		if p.IsGitHubOIDCProvider() {
			// GitHub OAuth Apps may return access_token-only responses in web flow.
			idClaims, err = githubClaimsFromAccessToken(ctx, u.hc, p.GitHub, strings.TrimSpace(tr.AccessToken))
			if err != nil {
				return out, err
			}
		} else {
			return out, fmt.Errorf("%w: %w", oidc.ErrTokenExchange, oidc.ErrMissingIDTokenInTokenResponse)
		}
	} else {
		nonceOpt := ""
		if strings.TrimSpace(intent.Nonce) != "" {
			nonceOpt = intent.Nonce
		}
		idClaims, err = oidc.ValidateIDToken(ctx, u.hc, disc, tr.IDToken, oidc.ValidateIDTokenOptions{
			ExpectedIssuer:   disc.Issuer,
			ExpectedAudience: p.ClientID,
			ExpectedNonce:    nonceOpt,
		})
		if err != nil {
			return out, err
		}
	}

	subject := interactiveSubject(idClaims)
	if subject == "" {
		return out, fmt.Errorf("%w: cannot derive subject from id_token (need email or sub)", apierrors.ErrInvalidInput)
	}

	snap, err := claimsSnapshotFromProvider(ctx, p, idClaims, strings.TrimSpace(tr.AccessToken), u.hc)
	if err != nil {
		return out, err
	}

	idpRT := strings.TrimSpace(tr.RefreshToken)
	env, err := u.sealer.Seal([]byte(idpRT))
	if err != nil {
		return out, err
	}
	envBytes, err := sessioncrypto.MarshalEnvelope(env)
	if err != nil {
		return out, err
	}

	now := time.Now().UTC()
	refreshCapTTL := u.tokenTTLPolicy.MaxRefreshTTL
	if refreshCapTTL <= 0 {
		refreshCapTTL = u.tokenTTLPolicy.DefaultRefreshTTL
	}
	if refreshCapTTL <= 0 {
		return out, fmt.Errorf("%w: invalid refresh ttl policy", apierrors.ErrInvalidInput)
	}
	refreshExpiresAt := initialRefreshExpiry(now, refreshCapTTL, tr)
	_, verifier, err := newOpaqueRefreshTokenAndVerifier()
	if err != nil {
		return out, err
	}

	sessionID := state
	sess := kvvalue.SessionValue{
		EncryptedIDPRefresh: json.RawMessage(envBytes),
		ClaimsSnapshot:      snap,
		RotationGeneration:  0,
		LoginIntentID:       state,
		ProviderID:          strings.TrimSpace(intent.ProviderID),
		OurRefreshVerifier:  verifier,
		RefreshExpiresAt:    refreshExpiresAt,
	}
	if err := u.sessions.Create(ctx, sessionID, sess); err != nil {
		return out, err
	}

	loc, err := oauthAuthorizeClientRedirect(intent.OAuthClientRedirectURI, state, intent.OAuthClientState)
	if err != nil {
		return out, fmt.Errorf("%w: oauth redirect uri: %s", apierrors.ErrInvalidInput, err.Error())
	}
	out.RedirectURL = loc
	out.AuthorizationID = state
	if u.idpCache != nil {
		at := strings.TrimSpace(tr.AccessToken)
		if at != "" {
			accessCapTTL := u.tokenTTLPolicy.MaxAccessTTL
			if accessCapTTL <= 0 {
				accessCapTTL = u.tokenTTLPolicy.DefaultAccessTTL
			}
			if accessCapTTL <= 0 {
				accessCapTTL = time.Minute
			}
			maxAccessExp := now.Add(accessCapTTL)
			if ttl, ok := idpcache.EntryTTL(u.idpCache.Now(), maxAccessExp, at, tr.ExpiresIn, u.idpOpaqueMaxTTL); ok {
				u.idpCache.Put(sessionID, at, ttl)
			}
		}
	}
	return out, nil
}

func isOAuthClientIntent(intent kvvalue.LoginIntentValue) bool {
	return strings.TrimSpace(intent.OAuthClientRedirectURI) != "" &&
		strings.TrimSpace(intent.OAuthClientCodeChallenge) != "" &&
		strings.EqualFold(strings.TrimSpace(intent.OAuthClientCodeChallengeMethod), "S256")
}

func oauthAuthorizeClientRedirect(redirectURI, code, state string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(redirectURI))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errors.New("invalid redirect_uri")
	}
	q := u.Query()
	q.Set("code", strings.TrimSpace(code))
	if strings.TrimSpace(state) != "" {
		q.Set("state", strings.TrimSpace(state))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func newOpaqueRefreshTokenAndVerifier() (token, verifier string, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("our refresh: %w", err)
	}
	token = hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	verifier = hex.EncodeToString(sum[:])
	return token, verifier, nil
}

func tokenExchangeProblemDetail(err error) string {
	if errors.Is(err, oidc.ErrMissingIDTokenInTokenResponse) {
		return "The IdP token response had no id_token (OIDC is required for this login flow). " +
			"On GitHub use Developer settings → OAuth Apps (classic) as in docs/idp/github—not only a GitHub App whose user-token response omits id_token. " +
			"Ensure the authorize request includes the openid scope and Accept: application/json on the token POST."
	}
	var te *oidc.OAuth2TokenError
	if errors.As(err, &te) {
		switch strings.ToLower(strings.TrimSpace(te.Code)) {
		case "redirect_uri_mismatch":
			return "The token request redirect_uri does not match the authorize step or the provider's registered callback rules."
		case "incorrect_client_credentials":
			return "The identity provider rejected client_id or client_secret for this OIDC provider entry."
		case "bad_verification_code":
			return "The identity provider rejected the authorization code (expired, already used, or PKCE / redirect_uri mismatch)."
		case "unverified_user_email":
			return "GitHub will not issue tokens until the user's primary email address is verified."
		default:
			c := strings.TrimSpace(te.Code)
			d := strings.TrimSpace(te.Description)
			if c != "" && d != "" {
				return c + ": " + d
			}
			if d != "" {
				return d
			}
			if c != "" {
				return c
			}
		}
	}
	return "Authorization code could not be exchanged (invalid or reused code)."
}

func interactiveSubject(mc jwt.MapClaims) string {
	if e, _ := mc["email"].(string); strings.TrimSpace(e) != "" {
		return strings.TrimSpace(e)
	}
	if p, _ := mc["preferred_username"].(string); strings.TrimSpace(p) != "" {
		return strings.TrimSpace(p)
	}
	if s, _ := mc["sub"].(string); strings.TrimSpace(s) != "" {
		return strings.TrimSpace(s)
	}
	return ""
}

// MapCallbackError maps Complete errors to HTTP status and stable problem codes.
func MapCallbackError(err error) (status int, code, detail string) {
	switch {
	case err == nil:
		return 0, "", ""
	case errors.Is(err, apierrors.ErrOIDCNotConfigured):
		return 400, "OIDC_NOT_CONFIGURED", "Configure auth.oidc_providers to enable browser login."
	case errors.Is(err, apierrors.ErrOIDCUnknownProvider):
		return 400, "OIDC_UNKNOWN_PROVIDER", "Login intent refers to an unknown provider_id."
	case errors.Is(err, apierrors.ErrNotFound):
		return 400, "OIDC_LOGIN_INTENT_NOT_FOUND", "Unknown or expired login state; start login again."
	case errors.Is(err, apierrors.ErrInvalidInput):
		return 400, "OIDC_CALLBACK_INVALID", err.Error()
	case errors.Is(err, oidc.ErrDiscovery):
		return 502, "OIDC_DISCOVERY_FAILED", "Could not load OpenID configuration from the issuer."
	case errors.Is(err, oidc.ErrTokenExchange):
		return 401, "OIDC_TOKEN_EXCHANGE_FAILED", tokenExchangeProblemDetail(err)
	case errors.Is(err, oidc.ErrIDTokenValidation):
		return 401, "OIDC_ID_TOKEN_INVALID", "IdP id_token validation failed."
	case errors.Is(err, apierrors.ErrGitHubLoginDenied):
		return 403, "GITHUB_LOGIN_DENIED", "GitHub user does not satisfy allowed organization membership for this provider."
	case errors.Is(err, apierrors.ErrGitLabLoginDenied):
		return 403, "GITLAB_LOGIN_DENIED", "GitLab user does not satisfy allowed group membership for this provider."
	case errors.Is(err, apierrors.ErrGoogleLoginDenied):
		return 403, "GOOGLE_LOGIN_DENIED", "Google user does not satisfy hosted domain or email domain policy for this provider."
	case errors.Is(err, apierrors.ErrOktaLoginDenied):
		return 403, "OKTA_LOGIN_DENIED", "Okta user id_token groups do not satisfy policy for this provider."
	case errors.Is(err, apierrors.ErrEntraLoginDenied):
		return 403, "ENTRA_LOGIN_DENIED", "Microsoft Entra user id_token tid/groups do not satisfy policy for this provider."
	case errors.Is(err, apierrors.ErrNoActiveSigningKey), errors.Is(err, apierrors.ErrUnsupportedSigningAlgorithm), errors.Is(err, apierrors.ErrSigningOperationFailed):
		return 503, "JWT_SIGNING_UNAVAILABLE", "Could not mint API access token."
	case errors.Is(err, apierrors.ErrStoreAccess):
		return 503, "STORE_UNAVAILABLE", "Could not persist session."
	default:
		return 500, "INTERNAL_ERROR", "Callback processing failed."
	}
}
