package auth

import (
	"net/url"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/config"
)

func TestRedirectURIAllowlisted(t *testing.T) {
	t.Parallel()
	allow := []string{" https://app/cb ", "http://127.0.0.1:9/x "}
	if !RedirectURIAllowlisted(allow, "https://app/cb") {
		t.Fatal("expected match with trim")
	}
	if RedirectURIAllowlisted(allow, "https://app/cb/") {
		t.Fatal("suffix must not match")
	}
	if RedirectURIAllowlisted(allow, "https://evil/cb") {
		t.Fatal("host must not match loosely")
	}
}

func TestMergeAuthorizeQuery(t *testing.T) {
	t.Parallel()
	add := url.Values{}
	add.Set("a", "1")
	add.Set("b", "2")
	out, err := mergeAuthorizeQuery("https://idp.example/authorize?existing=x", add)
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(out)
	if err != nil {
		t.Fatal(err)
	}
	if u.Query().Get("existing") != "x" {
		t.Fatalf("existing: %v", u.Query())
	}
	if u.Query().Get("a") != "1" || u.Query().Get("b") != "2" {
		t.Fatalf("merged: %v", u.RawQuery)
	}
}

func TestBuildOIDCScope(t *testing.T) {
	t.Parallel()
	s := buildOIDCScope(config.OIDCProviderConfig{
		ExtraScopes: []string{" email ", "", "profile"},
	})
	if s != "openid email profile" {
		t.Fatalf("got %q", s)
	}
	s2 := buildOIDCScope(config.OIDCProviderConfig{})
	if s2 != "openid" {
		t.Fatalf("got %q", s2)
	}
}
