package oidc

import "errors"

var (
	// ErrDiscovery is a failure fetching or parsing OpenID discovery.
	ErrDiscovery = errors.New("oidc: discovery failed")

	// ErrTokenExchange is a failure calling the token endpoint.
	ErrTokenExchange = errors.New("oidc: token exchange failed")

	// ErrIDTokenValidation is returned when id_token cannot be cryptographically verified or claims are invalid.
	ErrIDTokenValidation = errors.New("oidc: id_token validation failed")
)
