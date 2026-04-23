package oidc

import (
	"errors"
	"fmt"
)

var (
	// ErrDiscovery is a failure fetching or parsing OpenID discovery.
	ErrDiscovery = errors.New("oidc: discovery failed")

	// ErrTokenExchange is a failure calling the token endpoint.
	ErrTokenExchange = errors.New("oidc: token exchange failed")

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
