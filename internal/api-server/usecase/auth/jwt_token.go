package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// GenerateToken generates a JWT token.
func (uc *JWTUseCase) GenerateToken(ctx context.Context, req *models.GenerateTokenRequest) (*models.GenerateTokenResponse, error) {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseAuthPkg, "GenerateToken"))
	defer span.End()
	now := time.Now()
	tokenID := uuid.New().String()

	claims := jwt.MapClaims{
		"iss":          uc.edgeIssuer,
		"aud":          uc.edgeAudience,
		"sub":          req.AppID,
		"app_id":       req.AppID,
		"environments": req.Environments,
		"iat":          now.Unix(),
		"exp":          req.ExpiresAt.Unix(),
		"jti":          tokenID,
	}

	keyPair := uc.edgeSigningKeys[uc.edgeActiveKeyID]
	if keyPair == nil {
		err := fmt.Errorf("%w", apierrors.ErrNoActiveSigningKey)
		telemetry.MarkError(span, err)
		return nil, err
	}

	var token *jwt.Token
	switch keyPair.Algorithm {
	case AlgorithmEdDSA:
		token = jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	case AlgorithmRS256:
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	default:
		err := fmt.Errorf("%w: %s", apierrors.ErrUnsupportedSigningAlgorithm, keyPair.Algorithm)
		telemetry.MarkError(span, err)
		return nil, err
	}

	token.Header["kid"] = keyPair.Kid

	tokenString, err := token.SignedString(keyPair.PrivateKey)
	if err != nil {
		err = errors.Join(apierrors.ErrSigningOperationFailed, fmt.Errorf("jwt SignedString: %w", err))
		telemetry.MarkError(span, err)
		return nil, err
	}

	return &models.GenerateTokenResponse{
		ID:           tokenID,
		Token:        tokenString,
		AppID:        req.AppID,
		Environments: req.Environments,
		ExpiresAt:    req.ExpiresAt,
		CreatedAt:    now,
	}, nil
}

func parseInteractiveSnapshotForJWT(snapshot json.RawMessage) (roles []any, idpIss, idpSub string) {
	if len(snapshot) == 0 {
		return []any{}, "", ""
	}
	var m map[string]any
	if err := json.Unmarshal(snapshot, &m); err != nil {
		return []any{}, "", ""
	}
	if v, ok := m["roles"]; ok {
		if a, ok := v.([]any); ok {
			roles = a
		}
	}
	if roles == nil {
		roles = []any{}
	}
	if s, _ := m["idp_iss"].(string); strings.TrimSpace(s) != "" {
		idpIss = strings.TrimSpace(s)
	}
	if s, _ := m["idp_sub"].(string); strings.TrimSpace(s) != "" {
		idpSub = strings.TrimSpace(s)
	}
	return roles, idpIss, idpSub
}

// MintInteractiveAPIAccessJWT issues a short-lived API-profile access JWT after browser OIDC (roadmap ш. 14–15).
func (uc *JWTUseCase) MintInteractiveAPIAccessJWT(ctx context.Context, subject string, ttl time.Duration) (tokenStr, jti string, exp time.Time, err error) {
	return uc.MintInteractiveAPIAccessJWTFromSnapshot(ctx, subject, nil, ttl)
}

// MintInteractiveAPIAccessJWTFromSnapshot mints an API access JWT using session claims_snapshot for roles / idp_* (degraded refresh, roadmap ш. 18).
func (uc *JWTUseCase) MintInteractiveAPIAccessJWTFromSnapshot(ctx context.Context, subject string, snapshot json.RawMessage, ttl time.Duration) (tokenStr, jti string, exp time.Time, err error) {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseAuthPkg, "MintInteractiveAPIAccessJWTFromSnapshot"))
	defer span.End()
	now := time.Now()
	exp = now.Add(ttl)
	jti = uuid.New().String()

	roles, idpIss, idpSub := parseInteractiveSnapshotForJWT(snapshot)
	claims := jwt.MapClaims{
		"iss":   uc.apiIssuer,
		"sub":   subject,
		"aud":   uc.apiAudience,
		"iat":   now.Unix(),
		"exp":   exp.Unix(),
		"jti":   jti,
		"roles": roles,
	}
	if idpIss != "" {
		claims["idp_iss"] = idpIss
	}
	if idpSub != "" {
		claims["idp_sub"] = idpSub
	}

	keyPair := uc.apiSigningKeys[uc.apiActiveKeyID]
	if keyPair == nil {
		err = fmt.Errorf("%w", apierrors.ErrNoActiveSigningKey)
		telemetry.MarkError(span, err)
		return "", "", time.Time{}, err
	}

	var token *jwt.Token
	switch keyPair.Algorithm {
	case AlgorithmEdDSA:
		token = jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	case AlgorithmRS256:
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	default:
		err = fmt.Errorf("%w: %s", apierrors.ErrUnsupportedSigningAlgorithm, keyPair.Algorithm)
		telemetry.MarkError(span, err)
		return "", "", time.Time{}, err
	}
	token.Header["kid"] = keyPair.Kid

	tokenStr, err = token.SignedString(keyPair.PrivateKey)
	if err != nil {
		err = errors.Join(apierrors.ErrSigningOperationFailed, fmt.Errorf("jwt SignedString: %w", err))
		telemetry.MarkError(span, err)
		return "", "", time.Time{}, err
	}
	return tokenStr, jti, exp, nil
}
