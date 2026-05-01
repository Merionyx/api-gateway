package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type discoveryRoundTripFunc func(*http.Request) (*http.Response, error)

func (f discoveryRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

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

func TestFetchDiscovery_RejectsHTTPByDefault(t *testing.T) {
	t.Parallel()
	hc := &http.Client{Transport: discoveryRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		doc, _ := json.Marshal(Discovery{
			Issuer:                "http://idp.local",
			AuthorizationEndpoint: "http://idp.local/authorize",
			TokenEndpoint:         "http://idp.local/token",
			JWKSURI:               "http://idp.local/jwks",
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(doc))),
			Request:    r,
		}, nil
	})}

	_, err := FetchDiscovery(context.Background(), hc, "http://idp.local", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrDiscovery) || !errors.Is(err, ErrInsecureEndpoint) {
		t.Fatalf("got %v", err)
	}
}

func TestFetchDiscovery_AllowsHTTPWhenConfigured(t *testing.T) {
	t.Parallel()
	hc := &http.Client{Transport: discoveryRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		doc, _ := json.Marshal(Discovery{
			Issuer:                "http://idp.local",
			AuthorizationEndpoint: "http://idp.local/authorize",
			TokenEndpoint:         "http://idp.local/token",
			JWKSURI:               "http://idp.local/jwks",
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(doc))),
			Request:    r,
		}, nil
	})}

	disc, err := FetchDiscovery(context.Background(), hc, "http://idp.local", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if disc == nil || disc.TokenEndpoint == "" || disc.JWKSURI == "" {
		t.Fatalf("unexpected discovery: %+v", disc)
	}
}
