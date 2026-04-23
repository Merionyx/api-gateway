package jwt

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CachedToken struct {
	claims    jwt.MapClaims
	expiresAt time.Time
}

type JWTValidator struct {
	jwksURL    string
	publicKeys map[string]crypto.PublicKey // kid -> public key
	tokenCache sync.Map                    // map[string]CachedToken - cache of validated tokens
	mu         sync.RWMutex
	httpClient *http.Client
}

type JWKS struct {
	Keys []JWK `json:"keys"`
}

type JWK struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
}

func NewJWTValidator(jwksURL string) *JWTValidator {
	validator := &JWTValidator{
		jwksURL:    jwksURL,
		publicKeys: make(map[string]crypto.PublicKey),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}

	// Load keys on startup with retry
	maxRetries := 5
	backoff := time.Second

	for i := 0; i < maxRetries; i++ {
		if err := validator.RefreshKeys(); err != nil {
			slog.Warn("jwks: load keys attempt failed", "attempt", i+1, "max", maxRetries, "error", err)
			if i < maxRetries-1 {
				time.Sleep(backoff)
				backoff *= 2 // Exponential delay: 1s, 2s, 4s, 8s, 16s
			}
		} else {
			slog.Info("jwks: loaded keys", "attempt", i+1)
			break
		}
	}

	// Periodically refresh keys
	go validator.periodicRefresh()

	// Periodically clean up expired tokens from cache
	go validator.periodicCacheCleanup()

	return validator
}

// ValidateToken validates a JWT token with caching
func (v *JWTValidator) ValidateToken(tokenString string) (jwt.MapClaims, error) {
	// Check cache first
	if cached, ok := v.tokenCache.Load(tokenString); ok {
		cachedToken := cached.(CachedToken)
		// Check if token is still valid (not expired)
		if time.Now().Before(cachedToken.expiresAt) {
			return cachedToken.claims, nil
		}
		// Token expired, remove from cache
		v.tokenCache.Delete(tokenString)
	}

	// Cache miss or expired - validate the token
	claims, err := v.validateTokenInternal(tokenString)
	if err != nil {
		return nil, err
	}

	// Cache the validated token
	if exp, ok := claims["exp"].(float64); ok {
		expiresAt := time.Unix(int64(exp), 0)
		v.tokenCache.Store(tokenString, CachedToken{
			claims:    claims,
			expiresAt: expiresAt,
		})
	}

	return claims, nil
}

// validateTokenInternal performs the actual JWT validation (without caching)
func (v *JWTValidator) validateTokenInternal(tokenString string) (jwt.MapClaims, error) {
	// Parse the token to get the kid
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, fmt.Errorf("missing kid in token header")
	}

	// Get the public key
	publicKey := v.getPublicKey(kid)
	if publicKey == nil {
		// Try to update the keys
		if err := v.RefreshKeys(); err != nil {
			return nil, fmt.Errorf("failed to refresh keys: %w", err)
		}

		publicKey = v.getPublicKey(kid)
		if publicKey == nil {
			return nil, fmt.Errorf("public key not found for kid: %s", kid)
		}
	}

	// Validate the token
	parsedToken, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Check the algorithm
		switch token.Method.(type) {
		case *jwt.SigningMethodEd25519:
			// EdDSA
		case *jwt.SigningMethodRSA:
			// RSA
		default:
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok && parsedToken.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// RefreshKeys updates the public keys from the JWKS endpoint
func (v *JWTValidator) RefreshKeys() error {
	resp, err := v.httpClient.Get(v.jwksURL)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read JWKS response: %w", err)
	}

	var jwks JWKS
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("failed to unmarshal JWKS: %w", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	// Clear the old keys
	v.publicKeys = make(map[string]crypto.PublicKey)

	// Load the new keys
	for _, jwk := range jwks.Keys {
		publicKey, err := v.jwkToPublicKey(&jwk)
		if err != nil {
			slog.Warn("jwks: skip JWK", "kid", jwk.Kid, "error", err)
			continue
		}

		v.publicKeys[jwk.Kid] = publicKey
	}

	slog.Info("jwks: loaded public keys", "count", len(v.publicKeys))

	return nil
}

func (v *JWTValidator) getPublicKey(kid string) crypto.PublicKey {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.publicKeys[kid]
}

func (v *JWTValidator) periodicRefresh() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if err := v.RefreshKeys(); err != nil {
			slog.Warn("jwks: periodic refresh failed", "error", err)
		}
	}
}

// periodicCacheCleanup removes expired tokens from cache
func (v *JWTValidator) periodicCacheCleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		expiredCount := 0

		v.tokenCache.Range(func(key, value interface{}) bool {
			cached := value.(CachedToken)
			if now.After(cached.expiresAt) {
				v.tokenCache.Delete(key)
				expiredCount++
			}
			return true
		})

		if expiredCount > 0 {
			slog.Debug("JWT cache: cleaned expired tokens", "count", expiredCount)
		}
	}
}

// jwkToPublicKey converts a JWK to crypto.PublicKey
func (v *JWTValidator) jwkToPublicKey(jwk *JWK) (crypto.PublicKey, error) {
	switch jwk.Kty {
	case "OKP":
		// Ed25519
		if jwk.Crv != "Ed25519" {
			return nil, fmt.Errorf("unsupported curve: %s", jwk.Crv)
		}

		xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
		if err != nil {
			return nil, fmt.Errorf("failed to decode x: %w", err)
		}

		if len(xBytes) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("invalid Ed25519 public key size")
		}

		return ed25519.PublicKey(xBytes), nil

	case "RSA":
		// RSA
		nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
		if err != nil {
			return nil, fmt.Errorf("failed to decode n: %w", err)
		}

		eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
		if err != nil {
			return nil, fmt.Errorf("failed to decode e: %w", err)
		}

		n := new(big.Int).SetBytes(nBytes)
		e := new(big.Int).SetBytes(eBytes)

		return &rsa.PublicKey{
			N: n,
			E: int(e.Int64()),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported key type: %s", jwk.Kty)
	}
}
