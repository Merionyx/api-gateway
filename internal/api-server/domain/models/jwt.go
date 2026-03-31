package models

import "time"

// JWTToken represents a issued JWT token
type JWTToken struct {
	ID          string    `json:"id"`
	Token       string    `json:"token,omitempty"` // Shown only when creating
	Environment string    `json:"environment"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// GenerateTokenRequest request to generate a JWT token
type GenerateTokenRequest struct {
	AppID       string    `json:"app_id" validate:"required"`
	Environment string    `json:"environment" validate:"omitempty"`
	ExpiresAt   time.Time `json:"expires_at" validate:"required"`
}

// GenerateTokenResponse response with a JWT token
type GenerateTokenResponse struct {
	ID          string    `json:"id"`
	Token       string    `json:"token"` // Full JWT (shown only once!)
	AppID       string    `json:"app_id"`
	Environment string    `json:"environment,omitempty"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// JWK represents a JSON Web Key
type JWK struct {
	Kid string `json:"kid"`           // Key ID
	Kty string `json:"kty"`           // Key Type (RSA, OKP)
	Alg string `json:"alg"`           // Algorithm (RS256, EdDSA)
	Use string `json:"use,omitempty"` // Public Key Use (sig)

	// RSA specific
	N string `json:"n,omitempty"` // Modulus
	E string `json:"e,omitempty"` // Exponent

	// EdDSA specific
	Crv string `json:"crv,omitempty"` // Curve (Ed25519)
	X   string `json:"x,omitempty"`   // Public key
}

// JWKS represents a JSON Web Key Set
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// SigningKey represents a signing key
type SigningKey struct {
	Kid       string    `json:"kid"`
	Algorithm string    `json:"algorithm"` // EdDSA, RS256
	Active    bool      `json:"active"`    // Used for signing
	CreatedAt time.Time `json:"created_at"`
}
