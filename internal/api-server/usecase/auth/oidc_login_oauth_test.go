package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/auth/oidc"
	"github.com/merionyx/api-gateway/internal/api-server/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOIDCLoginUseCase_Start_OAuthAuthorizeMode(t *testing.T) {
	t.Parallel()

	stub := &stubIntentStore{}
	issuer := "https://issuer.example.com"
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			doc, _ := json.Marshal(oidc.Discovery{
				Issuer:                issuer,
				AuthorizationEndpoint: issuer + "/authorize",
				TokenEndpoint:         issuer + "/token",
				JWKSURI:               issuer + "/jwks",
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(string(doc))),
				Request:    r,
			}, nil
		}
		return &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("not found")), Request: r}, nil
	})}

	uc := NewOIDCLoginUseCase([]config.OIDCProviderConfig{{
		ID:                   "p1",
		Name:                 "Provider 1",
		Issuer:               issuer,
		ClientID:             "idp-client-id",
		RedirectURIAllowlist: []string{"https://oauth.pstmn.io/v1/callback"},
	}}, time.Minute, stub, hc, false)

	_, err := uc.Start(t.Context(), OIDCLoginStartRequest{
		RedirectURI:         "https://oauth.pstmn.io/v1/callback",
		ServerCallbackURI:   "https://api.example.com/v1/auth/callback",
		ResponseType:        "code",
		ClientID:            "postman-client",
		State:               "client-state-1",
		CodeChallenge:       "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~",
		CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := strings.TrimSpace(stub.last.RedirectURI); got != "https://api.example.com/v1/auth/callback" {
		t.Fatalf("idp redirect_uri=%q", got)
	}
	if got := strings.TrimSpace(stub.last.OAuthClientRedirectURI); got != "https://oauth.pstmn.io/v1/callback" {
		t.Fatalf("client redirect_uri=%q", got)
	}
	if got := strings.TrimSpace(stub.last.OAuthClientCodeChallengeMethod); got != "S256" {
		t.Fatalf("code challenge method=%q", got)
	}
	if got := strings.TrimSpace(stub.last.OAuthClientID); got != "postman-client" {
		t.Fatalf("client_id=%q", got)
	}
}
