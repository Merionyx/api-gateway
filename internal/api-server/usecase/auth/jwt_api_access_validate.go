package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ParseAndValidateAPIProfileBearerToken parses a Bearer JWT and verifies it against API signing keys, issuer, and audience (roadmap ш. 20).
// Edge-profile tokens fail (unknown kid / wrong iss / wrong aud).
func (uc *JWTUseCase) ParseAndValidateAPIProfileBearerToken(tokenString string) (jwt.MapClaims, error) {
	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return nil, errors.New("empty bearer token")
	}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg(), jwt.SigningMethodRS256.Alg()}),
		jwt.WithIssuer(uc.apiIssuer),
		jwt.WithAudience(uc.apiAudience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(10*time.Second),
	)
	var mc jwt.MapClaims
	_, err := parser.ParseWithClaims(tokenString, &mc, func(t *jwt.Token) (any, error) {
		if t.Method == nil {
			return nil, errors.New("jwt: missing signing method")
		}
		kid, _ := t.Header["kid"].(string)
		kid = strings.TrimSpace(kid)
		if kid == "" {
			return nil, errors.New("jwt: missing kid header")
		}
		kp := uc.apiSigningKeys[kid]
		if kp == nil {
			return nil, fmt.Errorf("jwt: unknown kid %q", kid)
		}
		switch kp.Algorithm {
		case AlgorithmEdDSA:
			if t.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
				return nil, fmt.Errorf("jwt: unexpected alg %q for EdDSA key", t.Method.Alg())
			}
		case AlgorithmRS256:
			if t.Method.Alg() != jwt.SigningMethodRS256.Alg() {
				return nil, fmt.Errorf("jwt: unexpected alg %q for RS256 key", t.Method.Alg())
			}
		default:
			return nil, fmt.Errorf("jwt: unsupported key algorithm %q", kp.Algorithm)
		}
		return kp.PublicKey, nil
	})
	if err != nil {
		return nil, err
	}
	return mc, nil
}
