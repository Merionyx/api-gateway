package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"sort"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/shared/telemetry"
	"go.opentelemetry.io/otel/trace"
)

// GetJWKS returns the API-profile JSON Web Key Set (HTTP API / interactive access verification).
func (uc *JWTUseCase) GetJWKS(ctx context.Context) (*models.JWKS, error) {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseAuthPkg, "GetJWKS"))
	defer span.End()
	return uc.buildJWKS(span, uc.apiSigningKeys)
}

// GetJWKSEdge returns the Edge-profile JWKS (data plane / ExtAuthz; POST /v1/tokens/edge).
func (uc *JWTUseCase) GetJWKSEdge(ctx context.Context) (*models.JWKS, error) {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseAuthPkg, "GetJWKSEdge"))
	defer span.End()
	return uc.buildJWKS(span, uc.edgeSigningKeys)
}

func (uc *JWTUseCase) buildJWKS(span trace.Span, signing map[string]*KeyPair) (*models.JWKS, error) {
	jwks := &models.JWKS{
		Keys: make([]models.JWK, 0),
	}

	kids := make([]string, 0, len(signing))
	for kid := range signing {
		kids = append(kids, kid)
	}
	sort.Strings(kids)

	for _, kid := range kids {
		keyPair := signing[kid]

		jwk, err := jwtPublicKeyToJWK(keyPair)
		if err != nil {
			err = fmt.Errorf("kid %s: %w", kid, err)
			telemetry.MarkError(span, err)
			return nil, err
		}

		jwks.Keys = append(jwks.Keys, *jwk)
	}

	return jwks, nil
}

// GetSigningKeys returns API-profile signing key metadata (GET /v1/keys).
func (uc *JWTUseCase) GetSigningKeys(ctx context.Context) []models.SigningKey {
	_, span := telemetry.Start(ctx, telemetry.SpanName(spanUsecaseAuthPkg, "GetSigningKeys"))
	defer span.End()
	keys := make([]models.SigningKey, 0, len(uc.apiSigningKeys))

	for kid, keyPair := range uc.apiSigningKeys {
		keys = append(keys, models.SigningKey{
			Kid:       kid,
			Algorithm: keyPair.Algorithm,
			Active:    kid == uc.apiActiveKeyID,
			CreatedAt: keyPair.CreatedAt,
		})
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].CreatedAt.After(keys[j].CreatedAt)
	})

	return keys
}

func jwtPublicKeyToJWK(keyPair *KeyPair) (*models.JWK, error) {
	jwk := &models.JWK{
		Kid: keyPair.Kid,
		Use: "sig",
	}

	switch keyPair.Algorithm {
	case AlgorithmEdDSA:
		edKey, ok := keyPair.PublicKey.(ed25519.PublicKey)
		if !ok {
			return nil, fmt.Errorf("%w: public key is not Ed25519", apierrors.ErrInvalidInput)
		}

		jwk.Kty = "OKP"
		jwk.Alg = "EdDSA"
		jwk.Crv = "Ed25519"
		jwk.X = base64.RawURLEncoding.EncodeToString(edKey)

	case AlgorithmRS256:
		rsaKey, ok := keyPair.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("%w: public key is not RSA", apierrors.ErrInvalidInput)
		}

		jwk.Kty = "RSA"
		jwk.Alg = "RS256"
		jwk.N = base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes())
		jwk.E = base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaKey.E)).Bytes())

	default:
		return nil, fmt.Errorf("%w: algorithm %q", apierrors.ErrUnsupportedSigningAlgorithm, keyPair.Algorithm)
	}

	return jwk, nil
}
