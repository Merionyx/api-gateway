package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// GenerateToken generates a JWT token.
func (uc *JWTUseCase) GenerateToken(req *models.GenerateTokenRequest) (*models.GenerateTokenResponse, error) {
	now := time.Now()
	tokenID := uuid.New().String()

	claims := jwt.MapClaims{
		"iss":          uc.issuer,
		"sub":          req.AppID,
		"app_id":       req.AppID,
		"environments": req.Environments,
		"iat":          now.Unix(),
		"exp":          req.ExpiresAt.Unix(),
		"jti":          tokenID,
	}

	keyPair := uc.signingKeys[uc.activeKeyID]
	if keyPair == nil {
		return nil, fmt.Errorf("%w", apierrors.ErrNoActiveSigningKey)
	}

	var token *jwt.Token
	switch keyPair.Algorithm {
	case AlgorithmEdDSA:
		token = jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	case AlgorithmRS256:
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	default:
		return nil, fmt.Errorf("%w: %s", apierrors.ErrUnsupportedSigningAlgorithm, keyPair.Algorithm)
	}

	token.Header["kid"] = keyPair.Kid

	tokenString, err := token.SignedString(keyPair.PrivateKey)
	if err != nil {
		return nil, errors.Join(apierrors.ErrSigningOperationFailed, fmt.Errorf("jwt SignedString: %w", err))
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
