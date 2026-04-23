package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/merionyx/api-gateway/internal/api-server/adapter/etcd"
	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/auth/oidc"
	"github.com/merionyx/api-gateway/internal/api-server/auth/sessioncrypto"
	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
)

// refreshSessionStore is implemented by etcd.SessionRepository (sessions + refresh-verifier index).
type refreshSessionStore interface {
	Get(ctx context.Context, sessionID string) (kvvalue.SessionValue, int64, error)
	GetSessionIDByRefreshVerifier(ctx context.Context, verifier string) (string, error)
	ReplaceCASWithRefreshIndex(ctx context.Context, sessionID, oldVerifier, newVerifier string, v kvvalue.SessionValue, expectedModRevision int64) error
}

// OIDCRefreshUseCase handles POST /api/v1/auth/refresh when IdP is reachable (roadmap ш. 17).
type OIDCRefreshUseCase struct {
	byID      map[string]config.OIDCProviderConfig
	sessions  refreshSessionStore
	sealer    *sessioncrypto.Keyring
	jwt       *JWTUseCase
	hc        *http.Client
	accessTTL time.Duration
}

// NewOIDCRefreshUseCase wires refresh; accessTTL<=0 defaults to 5m; hc nil uses http.DefaultClient.
func NewOIDCRefreshUseCase(
	providers []config.OIDCProviderConfig,
	sessions refreshSessionStore,
	sealer *sessioncrypto.Keyring,
	jwtUC *JWTUseCase,
	hc *http.Client,
	accessTTL time.Duration,
) *OIDCRefreshUseCase {
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
	return &OIDCRefreshUseCase{
		byID:      by,
		sessions:  sessions,
		sealer:    sealer,
		jwt:       jwtUC,
		hc:        hc,
		accessTTL: accessTTL,
	}
}

// ourOpaqueRefreshVerifier matches OIDCCallbackUseCase (SHA-256 of hex-encoded our refresh).
func ourOpaqueRefreshVerifier(ourRefreshHex string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(ourRefreshHex)))
	return hex.EncodeToString(sum[:])
}

func subjectFromClaimsSnapshot(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", errors.New("empty claims snapshot")
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", err
	}
	mc := jwt.MapClaims{}
	for _, k := range []string{"sub", "email", "name", "preferred_username"} {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				mc[k] = s
			}
		}
	}
	s := interactiveSubject(mc)
	if s == "" {
		return "", errors.New("no subject in claims snapshot")
	}
	return s, nil
}

func (u *OIDCRefreshUseCase) resolveProvider(sess kvvalue.SessionValue) (config.OIDCProviderConfig, error) {
	pid := strings.TrimSpace(sess.ProviderID)
	if pid != "" {
		p, ok := u.byID[pid]
		if !ok {
			return config.OIDCProviderConfig{}, apierrors.ErrOIDCUnknownProvider
		}
		return p, nil
	}
	if len(u.byID) == 1 {
		for _, p := range u.byID {
			return p, nil
		}
	}
	return config.OIDCProviderConfig{}, fmt.Errorf("%w: session missing provider_id", apierrors.ErrInvalidInput)
}

// Refresh rotates our refresh + IdP material when the IdP accepts refresh_token (IdP up branch).
func (u *OIDCRefreshUseCase) Refresh(ctx context.Context, ourRefreshHex string) (apiserver.AuthSessionTokensResponse, error) {
	var out apiserver.AuthSessionTokensResponse
	if len(u.byID) == 0 {
		return out, apierrors.ErrOIDCNotConfigured
	}
	if u.sealer == nil || u.sessions == nil || u.jwt == nil {
		return out, fmt.Errorf("%w: refresh dependencies not configured", apierrors.ErrInvalidInput)
	}

	ourRefreshHex = strings.TrimSpace(ourRefreshHex)
	if _, err := hex.DecodeString(ourRefreshHex); err != nil || len(ourRefreshHex) != 64 {
		return out, apierrors.ErrSessionAuthFailed
	}
	verifier := ourOpaqueRefreshVerifier(ourRefreshHex)

	sessionID, err := u.sessions.GetSessionIDByRefreshVerifier(ctx, verifier)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return out, apierrors.ErrSessionAuthFailed
		}
		return out, err
	}

	sess, modRev, err := u.sessions.Get(ctx, sessionID)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return out, apierrors.ErrSessionAuthFailed
		}
		return out, err
	}
	if subtle.ConstantTimeCompare([]byte(sess.OurRefreshVerifier), []byte(verifier)) != 1 {
		return out, apierrors.ErrSessionAuthFailed
	}

	p, err := u.resolveProvider(sess)
	if err != nil {
		return out, err
	}

	env, err := sessioncrypto.UnmarshalEnvelope([]byte(sess.EncryptedIDPRefresh))
	if err != nil {
		return out, fmt.Errorf("%w: session envelope", apierrors.ErrSessionAuthFailed)
	}
	idpRefreshBytes, err := u.sealer.Open(env)
	if err != nil {
		return out, fmt.Errorf("%w: cannot open session secrets", apierrors.ErrSessionAuthFailed)
	}
	idpRT := strings.TrimSpace(string(idpRefreshBytes))

	issuer := oidc.NormalizeIssuer(p.Issuer)
	disc, err := oidc.FetchDiscovery(ctx, u.hc, issuer)
	if err != nil {
		return out, fmt.Errorf("%w: %w", oidc.ErrDiscovery, err)
	}

	tr, err := oidc.ExchangeRefreshToken(ctx, u.hc, disc.TokenEndpoint, p.ClientID, p.ClientSecret, idpRT)
	if err != nil {
		return out, err
	}

	claimsSnap := sess.ClaimsSnapshot
	var subject string
	if strings.TrimSpace(tr.IDToken) != "" {
		idClaims, err := oidc.ValidateIDToken(ctx, u.hc, disc, tr.IDToken, oidc.ValidateIDTokenOptions{
			ExpectedIssuer:   disc.Issuer,
			ExpectedAudience: p.ClientID,
			ExpectedNonce:    "",
		})
		if err != nil {
			return out, err
		}
		snap, err := initialClaimsSnapshotJSON(idClaims)
		if err != nil {
			return out, err
		}
		claimsSnap = snap
		subject = interactiveSubject(idClaims)
	}
	if strings.TrimSpace(subject) == "" {
		subj, snapErr := subjectFromClaimsSnapshot(claimsSnap)
		if snapErr != nil {
			return out, apierrors.ErrSessionAuthFailed
		}
		subject = subj
	}

	accessJWT, _, _, err := u.jwt.MintInteractiveAPIAccessJWT(ctx, subject, u.accessTTL)
	if err != nil {
		return out, err
	}

	newIDPRT := strings.TrimSpace(tr.RefreshToken)
	if newIDPRT == "" {
		newIDPRT = idpRT
	}

	newOur := make([]byte, 32)
	if _, err := rand.Read(newOur); err != nil {
		return out, fmt.Errorf("our refresh: %w", err)
	}
	newOurStr := hex.EncodeToString(newOur)
	newSum := sha256.Sum256([]byte(newOurStr))
	newVerifier := hex.EncodeToString(newSum[:])

	sealed, err := u.sealer.Seal([]byte(newIDPRT))
	if err != nil {
		return out, err
	}
	envBytes, err := sessioncrypto.MarshalEnvelope(sealed)
	if err != nil {
		return out, err
	}

	newSess := kvvalue.SessionValue{
		EncryptedIDPRefresh: json.RawMessage(envBytes),
		ClaimsSnapshot:      claimsSnap,
		RotationGeneration:  sess.RotationGeneration + 1,
		LoginIntentID:       sess.LoginIntentID,
		ProviderID:          strings.TrimSpace(sess.ProviderID),
		OurRefreshVerifier:  newVerifier,
	}
	if newSess.ProviderID == "" && len(u.byID) == 1 {
		for id := range u.byID {
			newSess.ProviderID = id
			break
		}
	}

	if err := u.sessions.ReplaceCASWithRefreshIndex(ctx, sessionID, verifier, newVerifier, newSess, modRev); err != nil {
		if errors.Is(err, etcd.ErrSessionCASConflict) {
			return out, apierrors.ErrSessionRefreshConflict
		}
		return out, err
	}

	bt := "Bearer"
	out = apiserver.AuthSessionTokensResponse{
		AccessToken:  accessJWT,
		RefreshToken: newOurStr,
		TokenType:    &bt,
	}
	return out, nil
}

// MapRefreshError maps Refresh errors to HTTP status and stable problem codes.
func MapRefreshError(err error) (status int, code, detail string) {
	switch {
	case err == nil:
		return 0, "", ""
	case errors.Is(err, apierrors.ErrOIDCNotConfigured):
		return http.StatusBadRequest, "OIDC_NOT_CONFIGURED", "Configure auth.oidc_providers to enable refresh."
	case errors.Is(err, apierrors.ErrInvalidInput):
		return http.StatusBadRequest, "REFRESH_INVALID", err.Error()
	case errors.Is(err, apierrors.ErrOIDCUnknownProvider):
		return http.StatusBadRequest, "OIDC_UNKNOWN_PROVIDER", "Session refers to an unknown provider_id."
	case errors.Is(err, apierrors.ErrSessionAuthFailed):
		return http.StatusUnauthorized, "SESSION_AUTH_FAILED", "Refresh token is invalid, expired, or revoked."
	case errors.Is(err, apierrors.ErrSessionRefreshConflict):
		return http.StatusConflict, "REFRESH_STATE_CONFLICT", "Session state changed concurrently; retry with backoff or use the latest token pair from a successful refresh."
	case errors.Is(err, oidc.ErrDiscovery):
		return http.StatusBadGateway, "OIDC_DISCOVERY_FAILED", "Could not load OpenID configuration from the issuer."
	case errors.Is(err, oidc.ErrTokenExchange):
		return http.StatusUnauthorized, "OIDC_TOKEN_REFRESH_FAILED", "IdP rejected the refresh request."
	case errors.Is(err, oidc.ErrIDTokenValidation):
		return http.StatusUnauthorized, "OIDC_ID_TOKEN_INVALID", "IdP id_token validation failed after refresh."
	case errors.Is(err, apierrors.ErrNoActiveSigningKey), errors.Is(err, apierrors.ErrUnsupportedSigningAlgorithm), errors.Is(err, apierrors.ErrSigningOperationFailed):
		return http.StatusServiceUnavailable, "JWT_SIGNING_UNAVAILABLE", "Could not mint API access token."
	case errors.Is(err, apierrors.ErrStoreAccess):
		return http.StatusServiceUnavailable, "STORE_UNAVAILABLE", "Could not persist session."
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR", "Refresh processing failed."
	}
}
