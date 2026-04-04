package usecase

import (
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"sort"

	"merionyx/api-gateway/internal/api-server/domain/models"
)

// GetJWKS returns a JSON Web Key Set with all public keys.
func (uc *JWTUseCase) GetJWKS() (*models.JWKS, error) {
	jwks := &models.JWKS{
		Keys: make([]models.JWK, 0),
	}

	kids := make([]string, 0, len(uc.signingKeys))
	for kid := range uc.signingKeys {
		kids = append(kids, kid)
	}
	sort.Strings(kids)

	for _, kid := range kids {
		keyPair := uc.signingKeys[kid]

		jwk, err := jwtPublicKeyToJWK(keyPair)
		if err != nil {
			return nil, fmt.Errorf("failed to convert key %s to JWK: %w", kid, err)
		}

		jwks.Keys = append(jwks.Keys, *jwk)
	}

	return jwks, nil
}

// GetSigningKeys returns a list of signing keys.
func (uc *JWTUseCase) GetSigningKeys() []models.SigningKey {
	keys := make([]models.SigningKey, 0, len(uc.signingKeys))

	for kid, keyPair := range uc.signingKeys {
		keys = append(keys, models.SigningKey{
			Kid:       kid,
			Algorithm: keyPair.Algorithm,
			Active:    kid == uc.activeKeyID,
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
			return nil, fmt.Errorf("invalid Ed25519 public key")
		}

		jwk.Kty = "OKP"
		jwk.Alg = "EdDSA"
		jwk.Crv = "Ed25519"
		jwk.X = base64.RawURLEncoding.EncodeToString(edKey)

	case AlgorithmRS256:
		rsaKey, ok := keyPair.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("invalid RSA public key")
		}

		jwk.Kty = "RSA"
		jwk.Alg = "RS256"
		jwk.N = base64.RawURLEncoding.EncodeToString(rsaKey.N.Bytes())
		jwk.E = base64.RawURLEncoding.EncodeToString(big.NewInt(int64(rsaKey.E)).Bytes())

	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", keyPair.Algorithm)
	}

	return jwk, nil
}
