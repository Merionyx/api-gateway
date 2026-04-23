// Package pkce implements RFC 7636 PKCE (Proof Key for Code Exchange) for OAuth/OIDC authorization code flows.
package pkce

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const (
	// VerifierByteLen is the raw random length before base64url encoding (32 bytes → 43 chars, within RFC 7636 §4.1).
	VerifierByteLen = 32
)

// NewVerifier returns a high-entropy code_verifier string (base64url, no padding).
func NewVerifier() (string, error) {
	b := make([]byte, VerifierByteLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("pkce: random verifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ChallengeS256 returns the S256 code_challenge for a given code_verifier (base64url SHA-256, no padding).
func ChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
