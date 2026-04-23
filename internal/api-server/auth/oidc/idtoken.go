package oidc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ValidateIDTokenOptions configures id_token verification.
type ValidateIDTokenOptions struct {
	// ExpectedIssuer must match the "iss" claim (typically discovery.Issuer).
	ExpectedIssuer string
	// ExpectedAudience is required "aud" (string or first element if array).
	ExpectedAudience string
	// ExpectedNonce when non-empty must match the "nonce" claim.
	ExpectedNonce string
}

// ValidateIDToken verifies RS256 id_token signature using JWKS from discovery, then checks iss/aud/exp and optional nonce.
func ValidateIDToken(ctx context.Context, hc *http.Client, disc *Discovery, rawIDToken string, opts ValidateIDTokenOptions) (jwt.MapClaims, error) {
	if disc == nil {
		return nil, fmt.Errorf("%w: nil discovery", ErrIDTokenValidation)
	}
	keys, err := fetchRSAPublicKeys(ctx, hc, disc.JWKSURI)
	if err != nil {
		return nil, fmt.Errorf("%w: jwks: %w", ErrIDTokenValidation, err)
	}

	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	token, err := parser.Parse(rawIDToken, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, fmt.Errorf("unexpected alg %q", t.Method.Alg())
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("missing kid")
		}
		pub, ok := keys[kid]
		if !ok {
			return nil, fmt.Errorf("unknown kid %q", kid)
		}
		return pub, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIDTokenValidation, err)
	}
	mc, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("%w: claims type", ErrIDTokenValidation)
	}
	if err := validateStandardOIDCClaims(mc, opts); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIDTokenValidation, err)
	}
	return mc, nil
}

func validateStandardOIDCClaims(mc jwt.MapClaims, opts ValidateIDTokenOptions) error {
	if opts.ExpectedIssuer != "" {
		iss, _ := mc["iss"].(string)
		if iss != opts.ExpectedIssuer {
			return fmt.Errorf("iss mismatch")
		}
	}
	if opts.ExpectedAudience != "" {
		if !audienceContains(mc, opts.ExpectedAudience) {
			return fmt.Errorf("aud mismatch")
		}
	}
	if opts.ExpectedNonce != "" {
		nonce, _ := mc["nonce"].(string)
		if nonce != opts.ExpectedNonce {
			return fmt.Errorf("nonce mismatch")
		}
	}
	return nil
}

func audienceContains(mc jwt.MapClaims, want string) bool {
	switch aud := mc["aud"].(type) {
	case string:
		return aud == want
	case []any:
		for _, v := range aud {
			if s, ok := v.(string); ok && s == want {
				return true
			}
		}
	case []string:
		for _, s := range aud {
			if s == want {
				return true
			}
		}
	}
	return false
}

// NormalizeIssuer trims spaces and trailing slash for discovery URL construction.
func NormalizeIssuer(issuer string) string {
	return strings.TrimSuffix(strings.TrimSpace(issuer), "/")
}
