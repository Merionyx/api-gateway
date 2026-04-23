package oidc

import "testing"

func TestPatchWellKnownIncomplete_GitHub(t *testing.T) {
	t.Parallel()
	d := &Discovery{
		Issuer:  "https://github.com",
		JWKSURI: "https://github.com/login/oauth/.well-known/jwks",
	}
	patchWellKnownIncomplete(d)
	if d.AuthorizationEndpoint != "https://github.com/login/oauth/authorize" {
		t.Fatalf("AuthorizationEndpoint: %q", d.AuthorizationEndpoint)
	}
	if d.TokenEndpoint != "https://github.com/login/oauth/access_token" {
		t.Fatalf("TokenEndpoint: %q", d.TokenEndpoint)
	}
}

func TestPatchWellKnownIncomplete_NoOpForOtherIssuers(t *testing.T) {
	t.Parallel()
	d := &Discovery{
		Issuer:                "https://idp.example",
		AuthorizationEndpoint: "https://idp.example/oauth2/authorize",
		TokenEndpoint:         "https://idp.example/oauth2/token",
		JWKSURI:               "https://idp.example/jwks",
	}
	patchWellKnownIncomplete(d)
	if d.AuthorizationEndpoint != "https://idp.example/oauth2/authorize" {
		t.Fatal("unexpected patch")
	}
}
