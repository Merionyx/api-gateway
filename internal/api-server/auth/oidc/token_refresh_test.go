package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExchangeRefreshToken_ok(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), "grant_type=refresh_token") {
			http.Error(w, "grant", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"at","token_type":"Bearer","expires_in":3600}`))
	}))
	defer srv.Close()

	tr, err := ExchangeRefreshToken(context.Background(), srv.Client(), srv.URL, "cid", "sec", "rt", true)
	if err != nil {
		t.Fatal(err)
	}
	if tr.AccessToken != "at" {
		t.Fatalf("got %+v", tr)
	}
}

func TestExchangeRefreshToken_readsProviderRefreshLifetime(t *testing.T) {
	t.Parallel()
	var tr TokenResponse
	err := json.Unmarshal([]byte(`{"access_token":"at","token_type":"Bearer","expires_in":3600,"refresh_token_expires_in":86400,"refresh_expires_in":43200}`), &tr)
	if err != nil {
		t.Fatal(err)
	}
	if tr.RefreshTokenExpiresIn != 86400 || tr.RefreshExpiresIn != 43200 {
		t.Fatalf("got %+v", tr)
	}
}

func TestExchangeRefreshToken_503_degradable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := ExchangeRefreshToken(context.Background(), srv.Client(), srv.URL, "cid", "sec", "rt", true)
	if err == nil {
		t.Fatal("expected error")
	}
	var te *TokenExchangeFailure
	if !errors.As(err, &te) || te.HTTPStatus != http.StatusServiceUnavailable || !te.Degradable() {
		t.Fatalf("got %#v", err)
	}
	if !ShouldDegradeRefresh(err) {
		t.Fatal("expected ShouldDegradeRefresh")
	}
}

func TestExchangeRefreshToken_RejectsHTTPByDefault_NotDegradable(t *testing.T) {
	t.Parallel()
	_, err := ExchangeRefreshToken(context.Background(), http.DefaultClient, "http://idp.example/token", "cid", "sec", "rt", false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrTokenExchange) || !errors.Is(err, ErrInsecureEndpoint) {
		t.Fatalf("got %v", err)
	}
	if ShouldDegradeRefresh(err) {
		t.Fatal("insecure endpoint error must not trigger degraded refresh")
	}
}
