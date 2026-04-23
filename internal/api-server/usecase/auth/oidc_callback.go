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
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/merionyx/api-gateway/internal/api-server/auth/idpcache"
	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/auth/oidc"
	"github.com/merionyx/api-gateway/internal/api-server/auth/sessioncrypto"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
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

const defaultInteractiveAccessTTL = 5 * time.Minute

// OIDCCallbackUseCase completes GET /api/v1/auth/callback (roadmap ш. 14).
type OIDCCallbackUseCase struct {
	byID            map[string]config.OIDCProviderConfig
	intents         loginIntentReadDeleter
	sessions        sessionCreator
	sealer          envelopeSealer
	jwt             *JWTUseCase
	hc              *http.Client
	accessTTL       time.Duration
	idpCache        *idpcache.Cache
	idpOpaqueMaxTTL time.Duration
}

// NewOIDCCallbackUseCase wires callback dependencies. accessTTL<=0 defaults to 5m; hc nil uses http.DefaultClient.
func NewOIDCCallbackUseCase(
	providers []config.OIDCProviderConfig,
	intents loginIntentReadDeleter,
	sessions sessionCreator,
	sealer envelopeSealer,
	jwtUC *JWTUseCase,
	hc *http.Client,
	accessTTL time.Duration,
	idpCache *idpcache.Cache,
	idpOpaqueMaxTTL time.Duration,
) *OIDCCallbackUseCase {
	by := make(map[string]config.OIDCProviderConfig, len(providers))
	for _, p := range providers {
		by[strings.TrimSpace(p.ID)] = p
	}
	if accessTTL <= 0 {
		accessTTL = defaultInteractiveAccessTTL
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
		accessTTL:       accessTTL,
		idpCache:        idpCache,
		idpOpaqueMaxTTL: idpOpaqueMaxTTL,
	}
}

// Complete exchanges the authorization code, validates id_token, creates a session, and returns our token pair.
func (u *OIDCCallbackUseCase) Complete(ctx context.Context, code, state string) (apiserver.AuthSessionTokensResponse, error) {
	var out apiserver.AuthSessionTokensResponse
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

	nonceOpt := ""
	if strings.TrimSpace(intent.Nonce) != "" {
		nonceOpt = intent.Nonce
	}
	idClaims, err := oidc.ValidateIDToken(ctx, u.hc, disc, tr.IDToken, oidc.ValidateIDTokenOptions{
		ExpectedIssuer:   disc.Issuer,
		ExpectedAudience: p.ClientID,
		ExpectedNonce:    nonceOpt,
	})
	if err != nil {
		return out, err
	}

	subject := interactiveSubject(idClaims)
	if subject == "" {
		return out, fmt.Errorf("%w: cannot derive subject from id_token (need email or sub)", apierrors.ErrInvalidInput)
	}

	accessJWT, _, ourAccessExp, err := u.jwt.MintInteractiveAPIAccessJWT(ctx, subject, u.accessTTL)
	if err != nil {
		return out, err
	}

	ourRefresh := make([]byte, 32)
	if _, err := rand.Read(ourRefresh); err != nil {
		return out, fmt.Errorf("our refresh: %w", err)
	}
	ourRefreshStr := hex.EncodeToString(ourRefresh)
	sum := sha256.Sum256([]byte(ourRefreshStr))
	verifier := hex.EncodeToString(sum[:])

	idpRT := strings.TrimSpace(tr.RefreshToken)
	env, err := u.sealer.Seal([]byte(idpRT))
	if err != nil {
		return out, err
	}
	envBytes, err := sessioncrypto.MarshalEnvelope(env)
	if err != nil {
		return out, err
	}

	snap, err := claimsSnapshotFromProvider(ctx, p, idClaims, strings.TrimSpace(tr.AccessToken), u.hc)
	if err != nil {
		return out, err
	}

	sessionID := uuid.NewString()
	sess := kvvalue.SessionValue{
		EncryptedIDPRefresh: json.RawMessage(envBytes),
		ClaimsSnapshot:      snap,
		RotationGeneration:  0,
		LoginIntentID:       state,
		ProviderID:          strings.TrimSpace(intent.ProviderID),
		OurRefreshVerifier:  verifier,
	}
	if err := u.sessions.Create(ctx, sessionID, sess); err != nil {
		return out, err
	}

	if u.idpCache != nil {
		at := strings.TrimSpace(tr.AccessToken)
		if at != "" {
			if ttl, ok := idpcache.EntryTTL(u.idpCache.Now(), ourAccessExp, at, tr.ExpiresIn, u.idpOpaqueMaxTTL); ok {
				u.idpCache.Put(sessionID, at, ttl)
			}
		}
	}

	if err := u.intents.Delete(ctx, state); err != nil {
		return out, fmt.Errorf("login-intent delete: %w", err)
	}

	bt := "Bearer"
	out = apiserver.AuthSessionTokensResponse{
		AccessToken:  accessJWT,
		RefreshToken: ourRefreshStr,
		TokenType:    &bt,
	}
	return out, nil
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
		return 401, "OIDC_TOKEN_EXCHANGE_FAILED", "Authorization code could not be exchanged (invalid or reused code)."
	case errors.Is(err, oidc.ErrIDTokenValidation):
		return 401, "OIDC_ID_TOKEN_INVALID", "IdP id_token validation failed."
	case errors.Is(err, apierrors.ErrGitHubLoginDenied):
		return 403, "GITHUB_LOGIN_DENIED", "GitHub user does not satisfy allowed organization membership for this provider."
	case errors.Is(err, apierrors.ErrGitLabLoginDenied):
		return 403, "GITLAB_LOGIN_DENIED", "GitLab user does not satisfy allowed group membership for this provider."
	case errors.Is(err, apierrors.ErrNoActiveSigningKey), errors.Is(err, apierrors.ErrUnsupportedSigningAlgorithm), errors.Is(err, apierrors.ErrSigningOperationFailed):
		return 503, "JWT_SIGNING_UNAVAILABLE", "Could not mint API access token."
	case errors.Is(err, apierrors.ErrStoreAccess):
		return 503, "STORE_UNAVAILABLE", "Could not persist session."
	default:
		return 500, "INTERNAL_ERROR", "Callback processing failed."
	}
}
