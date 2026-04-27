package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	apiroles "github.com/merionyx/api-gateway/internal/api-server/auth/roles"
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

// interactiveSnapshotJWTInputs is the subset of claims_snapshot copied into API-profile access JWTs.
type interactiveSnapshotJWTInputs struct {
	Roles             []any
	OmitRoles         bool
	Permissions       []any
	IDPIss            string
	IDPSub            string
	Email             string
	PreferredUsername string
	Name              string
}

func snapshotStringClaim(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}

func parseInteractiveSnapshotForJWT(snapshot json.RawMessage) interactiveSnapshotJWTInputs {
	var out interactiveSnapshotJWTInputs
	out.Roles = []any{apiroles.APIRoleViewer}
	if len(snapshot) == 0 {
		return out
	}
	var m map[string]any
	if err := json.Unmarshal(snapshot, &m); err != nil {
		return out
	}
	if omit, _ := m["omit_roles"].(bool); omit {
		out.OmitRoles = true
		out.Roles = nil
	}
	if v, ok := m["roles"]; ok {
		out.Roles = claimAnyList(v)
		out.OmitRoles = false
	}
	if out.OmitRoles {
		out.Roles = nil
	} else if out.Roles == nil {
		out.Roles = []any{}
	}
	if _, ok := m["roles"]; !ok && !out.OmitRoles {
		out.Roles = []any{apiroles.APIRoleViewer}
	}
	if v, ok := m["permissions"]; ok {
		out.Permissions = claimAnyList(v)
	}
	out.IDPIss = snapshotStringClaim(m, "idp_iss")
	out.IDPSub = snapshotStringClaim(m, "idp_sub")
	out.Email = snapshotStringClaim(m, "email")
	out.PreferredUsername = snapshotStringClaim(m, "preferred_username")
	out.Name = snapshotStringClaim(m, "name")
	return out
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

	parts := parseInteractiveSnapshotForJWT(snapshot)
	claims := jwt.MapClaims{
		"iss": uc.apiIssuer,
		"sub": subject,
		"aud": uc.apiAudience,
		"iat": now.Unix(),
		"exp": exp.Unix(),
		"jti": jti,
	}
	if !parts.OmitRoles {
		claims["roles"] = parts.Roles
	}
	if len(parts.Permissions) > 0 {
		claims["permissions"] = parts.Permissions
	}
	if parts.IDPIss != "" {
		claims["idp_iss"] = parts.IDPIss
	}
	if parts.IDPSub != "" {
		claims["idp_sub"] = parts.IDPSub
	}
	if parts.Email != "" {
		claims["email"] = parts.Email
	}
	if parts.PreferredUsername != "" {
		claims["preferred_username"] = parts.PreferredUsername
	}
	if parts.Name != "" {
		claims["name"] = parts.Name
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

func claimAnyList(v any) []any {
	switch x := v.(type) {
	case []any:
		return append([]any(nil), x...)
	case []string:
		out := make([]any, 0, len(x))
		for i := range x {
			s := strings.TrimSpace(x[i])
			if s == "" {
				continue
			}
			out = append(out, s)
		}
		return out
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return []any{}
		}
		return []any{s}
	default:
		return []any{}
	}
}
