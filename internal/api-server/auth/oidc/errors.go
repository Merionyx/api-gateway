package oidc

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrDiscovery is a failure fetching or parsing OpenID discovery.
	ErrDiscovery = errors.New("oidc: discovery failed")

	// ErrTokenExchange is a failure calling the token endpoint.
	ErrTokenExchange = errors.New("oidc: token exchange failed")

	// ErrMissingIDTokenInTokenResponse is returned when the token endpoint returns HTTP 200 JSON without id_token
	// (authorization code flow). Some GitHub App user-token responses only include access_token.
	ErrMissingIDTokenInTokenResponse = errors.New("oidc: token response missing id_token")

	// ErrMissingRefreshTokenInTokenResponse is returned when the login flow completed without an IdP refresh token,
	// so the API Server cannot perform IdP-up refresh later.
	ErrMissingRefreshTokenInTokenResponse = errors.New("oidc: token response missing refresh_token")

	// ErrIDTokenValidation is returned when id_token cannot be cryptographically verified or claims are invalid.
	ErrIDTokenValidation = errors.New("oidc: id_token validation failed")
)

// TokenExchangeFailure wraps a non-success token endpoint response or transport error (refresh / code exchange).
type TokenExchangeFailure struct {
	HTTPStatus int   // 0 if no HTTP response (transport failure)
	Cause      error // set when HTTPStatus==0
}

func (e *TokenExchangeFailure) Error() string {
	if e.HTTPStatus == 0 {
		return fmt.Sprintf("%v: transport: %v", ErrTokenExchange, e.Cause)
	}
	return fmt.Sprintf("%v: status %d", ErrTokenExchange, e.HTTPStatus)
}

// Unwrap returns ErrTokenExchange for errors.Is classification.
func (e *TokenExchangeFailure) Unwrap() error { return ErrTokenExchange }

// OAuth2TokenError is returned when the token endpoint responds with HTTP 200 and an OAuth 2.0
// error JSON body (e.g. GitHub: redirect_uri_mismatch, bad_verification_code) instead of tokens.
type OAuth2TokenError struct {
	Code        string
	Description string
}

func (e *OAuth2TokenError) Error() string {
	if e == nil {
		return ""
	}
	code := strings.TrimSpace(e.Code)
	desc := strings.TrimSpace(e.Description)
	if code != "" && desc != "" {
		return fmt.Sprintf("%v: %s: %s", ErrTokenExchange, code, desc)
	}
	if code != "" {
		return fmt.Sprintf("%v: %s", ErrTokenExchange, code)
	}
	return ErrTokenExchange.Error()
}

// Unwrap returns ErrTokenExchange so errors.Is(err, ErrTokenExchange) matches.
func (e *OAuth2TokenError) Unwrap() error { return ErrTokenExchange }

// Degradable reports whether the API Server may use degraded refresh (IdP unreachable; roadmap ш. 18).
func (e *TokenExchangeFailure) Degradable() bool {
	return e.HTTPStatus == 0 || e.HTTPStatus >= 500
}

// ShouldDegradeRefresh is true for discovery failures or token transport / HTTP 5xx (not 4xx client errors).
func ShouldDegradeRefresh(err error) bool {
	if errors.Is(err, ErrDiscovery) {
		return true
	}
	var te *TokenExchangeFailure
	if errors.As(err, &te) {
		return te.Degradable()
	}
	return false
}
