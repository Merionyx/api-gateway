package auth

import (
	"crypto"
	"fmt"
	"strings"
	"time"
)

const (
	AlgorithmEdDSA = "EdDSA"
	AlgorithmRS256 = "RS256"
)

// JWTUseCase issues app tokens and exposes JWKS for verification (e.g. sidecar).
type JWTUseCase struct {
	keysDir       string
	issuer        string
	apiAudience   string
	signingKeys   map[string]*KeyPair // kid -> KeyPair
	activeKeyID   string
	activeKeyAlg  string
}

// KeyPair holds a signing identity loaded from disk or generated at startup.
type KeyPair struct {
	Kid        string
	Algorithm  string
	PrivateKey crypto.PrivateKey
	PublicKey  crypto.PublicKey
	CreatedAt  time.Time
}

// NewJWTUseCase loads signing keys. apiAudience is the aud claim for interactive API JWTs (see jwt.api_audience); empty defaults to issuer+"#api".
func NewJWTUseCase(keysDir, issuer, apiAudience string) (*JWTUseCase, error) {
	aud := strings.TrimSpace(apiAudience)
	if aud == "" {
		aud = strings.TrimSpace(issuer) + "#api"
	}
	uc := &JWTUseCase{
		keysDir:      keysDir,
		issuer:       issuer,
		apiAudience:  aud,
		signingKeys:  make(map[string]*KeyPair),
	}

	if err := uc.loadKeys(); err != nil {
		return nil, fmt.Errorf("failed to load keys: %w", err)
	}

	if len(uc.signingKeys) == 0 {
		if err := uc.generateDefaultKey(); err != nil {
			return nil, fmt.Errorf("failed to generate default key: %w", err)
		}
	}

	return uc, nil
}
