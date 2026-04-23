package config

import (
	"strings"
	"testing"
)

func TestValidateOIDCProviders_GitHub(t *testing.T) {
	t.Parallel()
	err := ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "gh",
		Issuer:               "https://example.com",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "github",
	}})
	if err == nil || !strings.Contains(err.Error(), "kind=github requires issuer") {
		t.Fatalf("expected issuer validation error, got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "gh",
		Issuer:               GitHubOIDCDiscoveryIssuer,
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "github",
		GitHub: &GitHubOIDCProviderConfig{
			TeamRoleBindings: []GitHubTeamRoleBinding{{
				Org: "acme", TeamSlug: "t", Roles: []string{},
			}},
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "roles must be non-empty") {
		t.Fatalf("expected team binding roles error, got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "gh",
		Issuer:               GitHubOIDCDiscoveryIssuer,
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "github",
		GitHub: &GitHubOIDCProviderConfig{
			AllowedOrgLogins: []string{"acme"},
			TeamRoleBindings: []GitHubTeamRoleBinding{{
				Org: "acme", TeamSlug: "platform", Roles: []string{"api:contracts:export"},
			}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAuthConfig_BootstrapAPIKeyAllowed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		cfg    AuthConfig
		allow  bool
	}{
		{"off", AuthConfig{AllowInsecureBootstrap: false, Environment: "development"}, false},
		{"prod flag on still blocked", AuthConfig{AllowInsecureBootstrap: true, Environment: "production"}, false},
		{"staging blocked", AuthConfig{AllowInsecureBootstrap: true, Environment: "staging"}, false},
		{"dev ok", AuthConfig{AllowInsecureBootstrap: true, Environment: "development"}, true},
		{"local ok", AuthConfig{AllowInsecureBootstrap: true, Environment: "local"}, true},
		{"dev case", AuthConfig{AllowInsecureBootstrap: true, Environment: "Development"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.cfg.BootstrapAPIKeyAllowed(); got != tc.allow {
				t.Fatalf("BootstrapAPIKeyAllowed()=%v want %v", got, tc.allow)
			}
		})
	}
}
