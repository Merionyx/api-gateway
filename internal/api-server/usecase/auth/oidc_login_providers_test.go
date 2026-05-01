package auth

import (
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/config"
)

func TestOIDCLoginUseCase_ListPublicOIDCProviders_sortedAndKind(t *testing.T) {
	t.Parallel()
	uc := NewOIDCLoginUseCase([]config.OIDCProviderConfig{
		{ID: "z", Name: "GitHub Enterprise", Issuer: "https://z.example", ClientID: "a", RedirectURIAllowlist: []string{"http://localhost/cb"}, Kind: "GitHub"},
		{ID: "a", Name: "Local OIDC", Issuer: "https://a.example", ClientID: "b", RedirectURIAllowlist: []string{"http://localhost/cb"}},
	}, 0, nil, nil, false)
	got := uc.ListPublicOIDCProviders()
	if len(got) != 2 {
		t.Fatalf("len=%d %+v", len(got), got)
	}
	if got[0].ID != "a" || got[0].Name != "Local OIDC" || got[0].Kind != "generic" || got[0].Issuer != "https://a.example" {
		t.Fatalf("first %+v", got[0])
	}
	if got[1].ID != "z" || got[1].Name != "GitHub Enterprise" || got[1].Kind != "github" {
		t.Fatalf("second %+v", got[1])
	}
}

func TestOIDCLoginUseCase_ListPublicOIDCProviders_empty(t *testing.T) {
	t.Parallel()
	uc := NewOIDCLoginUseCase(nil, 0, nil, nil, false)
	if uc.ListPublicOIDCProviders() != nil {
		t.Fatal("want nil slice")
	}
}
