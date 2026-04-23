package oidc

import (
	"context"
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

	tr, err := ExchangeRefreshToken(context.Background(), srv.Client(), srv.URL, "cid", "sec", "rt")
	if err != nil {
		t.Fatal(err)
	}
	if tr.AccessToken != "at" {
		t.Fatalf("got %+v", tr)
	}
}

func TestExchangeRefreshToken_503_degradable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := ExchangeRefreshToken(context.Background(), srv.Client(), srv.URL, "cid", "sec", "rt")
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
