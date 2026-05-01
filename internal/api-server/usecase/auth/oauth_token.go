package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/adapter/etcd"
	"github.com/merionyx/api-gateway/internal/api-server/auth/kvvalue"
	"github.com/merionyx/api-gateway/internal/api-server/auth/oidc"
	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
)

type oauthTokenIntentStore interface {
	Get(ctx context.Context, intentID string) (kvvalue.LoginIntentValue, error)
	Delete(ctx context.Context, intentID string) error
}

type oauthTokenSessionStore interface {
	Get(ctx context.Context, sessionID string) (kvvalue.SessionValue, int64, error)
	ReplaceCASWithRefreshIndex(ctx context.Context, sessionID, oldVerifier, newVerifier string, v kvvalue.SessionValue, expectedModRevision int64) error
}

type OAuthTokenRequest struct {
	GrantType    string
	Code         string
	RedirectURI  string
	ClientID     string
	CodeVerifier string
	RefreshToken string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
}

type OAuthTokenUseCase struct {
	intents        oauthTokenIntentStore
	sessions       oauthTokenSessionStore
	jwt            *JWTUseCase
	refreshUC      *OIDCRefreshUseCase
	tokenTTLPolicy TokenTTLPolicy
}

func NewOAuthTokenUseCase(
	intents oauthTokenIntentStore,
	sessions oauthTokenSessionStore,
	jwtUC *JWTUseCase,
	refreshUC *OIDCRefreshUseCase,
	tokenTTLPolicy TokenTTLPolicy,
) *OAuthTokenUseCase {
	return &OAuthTokenUseCase{
		intents:        intents,
		sessions:       sessions,
		jwt:            jwtUC,
		refreshUC:      refreshUC,
		tokenTTLPolicy: tokenTTLPolicy,
	}
}

func (u *OAuthTokenUseCase) Exchange(ctx context.Context, req OAuthTokenRequest) (apiserver.OAuthTokenResponse, error) {
	switch strings.ToLower(strings.TrimSpace(req.GrantType)) {
	case "authorization_code":
		return u.exchangeAuthorizationCode(ctx, req)
	case "refresh_token":
		return u.exchangeRefreshToken(ctx, req)
	default:
		return apiserver.OAuthTokenResponse{}, fmt.Errorf("%w: unsupported grant_type", apierrors.ErrInvalidInput)
	}
}

func (u *OAuthTokenUseCase) exchangeAuthorizationCode(ctx context.Context, req OAuthTokenRequest) (apiserver.OAuthTokenResponse, error) {
	if u.intents == nil || u.sessions == nil || u.jwt == nil {
		return apiserver.OAuthTokenResponse{}, fmt.Errorf("%w: oauth token dependencies not configured", apierrors.ErrInvalidInput)
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return apiserver.OAuthTokenResponse{}, fmt.Errorf("%w: code is required", apierrors.ErrInvalidInput)
	}
	redirectURI := strings.TrimSpace(req.RedirectURI)
	if redirectURI == "" {
		return apiserver.OAuthTokenResponse{}, fmt.Errorf("%w: redirect_uri is required", apierrors.ErrInvalidInput)
	}
	clientID := strings.TrimSpace(req.ClientID)
	if clientID == "" {
		return apiserver.OAuthTokenResponse{}, fmt.Errorf("%w: client_id is required", apierrors.ErrInvalidInput)
	}
	codeVerifier := strings.TrimSpace(req.CodeVerifier)
	if codeVerifier == "" {
		return apiserver.OAuthTokenResponse{}, fmt.Errorf("%w: code_verifier is required", apierrors.ErrInvalidInput)
	}

	intent, err := u.intents.Get(ctx, code)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return apiserver.OAuthTokenResponse{}, apierrors.ErrSessionAuthFailed
		}
		return apiserver.OAuthTokenResponse{}, err
	}
	if !isOAuthClientIntent(intent) {
		return apiserver.OAuthTokenResponse{}, apierrors.ErrSessionAuthFailed
	}
	if subtleTrimEqual(intent.OAuthClientID, clientID) == 0 {
		return apiserver.OAuthTokenResponse{}, apierrors.ErrSessionAuthFailed
	}
	if subtleTrimEqual(intent.OAuthClientRedirectURI, redirectURI) == 0 {
		return apiserver.OAuthTokenResponse{}, apierrors.ErrSessionAuthFailed
	}
	if !strings.EqualFold(strings.TrimSpace(intent.OAuthClientCodeChallengeMethod), "S256") {
		return apiserver.OAuthTokenResponse{}, apierrors.ErrSessionAuthFailed
	}
	if !pkceVerifierMatchesS256(codeVerifier, intent.OAuthClientCodeChallenge) {
		return apiserver.OAuthTokenResponse{}, apierrors.ErrSessionAuthFailed
	}

	sess, modRev, err := u.sessions.Get(ctx, code)
	if err != nil {
		if errors.Is(err, apierrors.ErrNotFound) {
			return apiserver.OAuthTokenResponse{}, apierrors.ErrSessionAuthFailed
		}
		return apiserver.OAuthTokenResponse{}, err
	}
	if subtleTrimEqual(sess.LoginIntentID, code) == 0 {
		return apiserver.OAuthTokenResponse{}, apierrors.ErrSessionAuthFailed
	}
	if sess.RotationGeneration != 0 {
		return apiserver.OAuthTokenResponse{}, apierrors.ErrSessionAuthFailed
	}
	if refreshSessionExpired(time.Now().UTC(), sess.RefreshExpiresAt) {
		return apiserver.OAuthTokenResponse{}, apierrors.ErrSessionRefreshExpired
	}

	resolvedTTLs, err := resolveRequestedTokenTTLs(u.tokenTTLPolicy, RequestedTokenTTLs{
		AccessTTL:  req.AccessTTL,
		RefreshTTL: req.RefreshTTL,
	})
	if err != nil {
		return apiserver.OAuthTokenResponse{}, fmt.Errorf("%w: %s", apierrors.ErrInvalidInput, err.Error())
	}
	now := time.Now().UTC()
	refreshExpiresAt := sessionRefreshExpiryFromRequest(now, sess.RefreshExpiresAt, resolvedTTLs.RefreshTTL)
	if !refreshExpiresAt.After(now) {
		return apiserver.OAuthTokenResponse{}, apierrors.ErrSessionRefreshExpired
	}

	subject, err := subjectFromClaimsSnapshot(sess.ClaimsSnapshot)
	if err != nil {
		return apiserver.OAuthTokenResponse{}, apierrors.ErrSessionAuthFailed
	}
	accessJWT, _, accessExp, err := u.jwt.MintInteractiveAPIAccessJWTFromSnapshot(ctx, subject, sess.ClaimsSnapshot, resolvedTTLs.AccessTTL)
	if err != nil {
		return apiserver.OAuthTokenResponse{}, err
	}

	refreshToken, newVerifier, err := newOpaqueRefreshTokenAndVerifier()
	if err != nil {
		return apiserver.OAuthTokenResponse{}, err
	}
	newSess := kvvalue.SessionValue{
		EncryptedIDPRefresh: cloneJSONRaw(sess.EncryptedIDPRefresh),
		ClaimsSnapshot:      cloneJSONRaw(sess.ClaimsSnapshot),
		RotationGeneration:  sess.RotationGeneration + 1,
		LoginIntentID:       sess.LoginIntentID,
		ProviderID:          strings.TrimSpace(sess.ProviderID),
		OurRefreshVerifier:  newVerifier,
		RefreshExpiresAt:    refreshExpiresAt,
	}
	if err := u.sessions.ReplaceCASWithRefreshIndex(ctx, code, sess.OurRefreshVerifier, newVerifier, newSess, modRev); err != nil {
		if errors.Is(err, etcd.ErrSessionCASConflict) {
			return apiserver.OAuthTokenResponse{}, apierrors.ErrSessionAuthFailed
		}
		return apiserver.OAuthTokenResponse{}, err
	}
	_ = u.intents.Delete(ctx, code)

	return oauthTokenResponse(accessJWT, accessExp, refreshToken, refreshExpiresAt), nil
}

func (u *OAuthTokenUseCase) exchangeRefreshToken(ctx context.Context, req OAuthTokenRequest) (apiserver.OAuthTokenResponse, error) {
	if u.refreshUC == nil {
		return apiserver.OAuthTokenResponse{}, fmt.Errorf("%w: refresh use case not configured", apierrors.ErrInvalidInput)
	}
	out, err := u.refreshUC.Refresh(ctx, OIDCRefreshRequest{
		RefreshToken:        strings.TrimSpace(req.RefreshToken),
		RequestedAccessTTL:  req.AccessTTL,
		RequestedRefreshTTL: req.RefreshTTL,
	})
	if err != nil {
		return apiserver.OAuthTokenResponse{}, err
	}
	return oauthTokenResponse(out.AccessToken, out.AccessExpiresAt, out.RefreshToken, out.RefreshExpiresAt), nil
}

func sessionRefreshExpiryFromRequest(now time.Time, absoluteCap time.Time, requestedTTL time.Duration) time.Time {
	expiresAt := now.Add(requestedTTL)
	if absoluteCap.IsZero() {
		return expiresAt
	}
	if expiresAt.Before(absoluteCap) {
		return expiresAt
	}
	return absoluteCap
}

func oauthTokenResponse(accessToken string, accessExpiresAt time.Time, refreshToken string, refreshExpiresAt time.Time) apiserver.OAuthTokenResponse {
	expiresIn := int(time.Until(accessExpiresAt).Seconds())
	if expiresIn < 1 {
		expiresIn = 1
	}
	refreshExpiresIn := int(time.Until(refreshExpiresAt).Seconds())
	if refreshExpiresIn < 1 {
		refreshExpiresIn = 1
	}
	bt := "Bearer"
	return apiserver.OAuthTokenResponse{
		AccessToken:      accessToken,
		TokenType:        bt,
		ExpiresIn:        expiresIn,
		RefreshToken:     &refreshToken,
		RefreshExpiresIn: &refreshExpiresIn,
		AccessExpiresAt:  &accessExpiresAt,
		RefreshExpiresAt: &refreshExpiresAt,
	}
}

func pkceVerifierMatchesS256(verifier, challenge string) bool {
	v := strings.TrimSpace(verifier)
	c := strings.TrimSpace(challenge)
	if v == "" || c == "" {
		return false
	}
	sum := sha256.Sum256([]byte(v))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtleTrimEqual(want, c) == 1
}

func subtleTrimEqual(a, b string) int {
	aa := strings.TrimSpace(a)
	bb := strings.TrimSpace(b)
	if len(aa) != len(bb) {
		return 0
	}
	var diff byte
	for i := 0; i < len(aa); i++ {
		diff |= aa[i] ^ bb[i]
	}
	if diff == 0 {
		return 1
	}
	return 0
}

func MapOAuthTokenError(err error) (status int, oauthError, description string) {
	switch {
	case err == nil:
		return 0, "", ""
	case errors.Is(err, apierrors.ErrInvalidInput):
		return http.StatusBadRequest, "invalid_request", err.Error()
	case errors.Is(err, apierrors.ErrSessionAuthFailed), errors.Is(err, apierrors.ErrSessionRefreshExpired), errors.Is(err, apierrors.ErrNotFound):
		return http.StatusBadRequest, "invalid_grant", "authorization code or refresh token is invalid, expired, or already used"
	case errors.Is(err, apierrors.ErrOIDCNotConfigured):
		return http.StatusBadRequest, "invalid_request", "oidc providers are not configured"
	case errors.Is(err, oidc.ErrTokenExchange), errors.Is(err, oidc.ErrIDTokenValidation):
		return http.StatusBadRequest, "invalid_grant", "identity provider rejected token exchange"
	case errors.Is(err, oidc.ErrDiscovery), errors.Is(err, apierrors.ErrStoreAccess),
		errors.Is(err, apierrors.ErrNoActiveSigningKey), errors.Is(err, apierrors.ErrUnsupportedSigningAlgorithm), errors.Is(err, apierrors.ErrSigningOperationFailed):
		return http.StatusServiceUnavailable, "temporarily_unavailable", "token endpoint is temporarily unavailable"
	default:
		return http.StatusInternalServerError, "server_error", "internal token endpoint error"
	}
}
