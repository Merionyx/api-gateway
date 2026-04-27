package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateOIDCProviders_GitHub(t *testing.T) {
	t.Parallel()
	err := ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "gh",
		Name:                 "GitHub",
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
		Name:                 "GitHub",
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
		Name:                 "GitHub",
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

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "gh",
		Name:                 "GitHub",
		Issuer:               GitHubOIDCDiscoveryIssuer,
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "github",
		GitHub:               &GitHubOIDCProviderConfig{AuthFlow: "bad"},
	}})
	if err == nil || !strings.Contains(err.Error(), "auth_flow") {
		t.Fatalf("expected auth_flow validation error, got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "gh",
		Name:                 "GitHub",
		Issuer:               GitHubOIDCDiscoveryIssuer,
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "github",
		GitHub:               &GitHubOIDCProviderConfig{AuthFlow: "github_app"},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateOIDCProviders_GitLab(t *testing.T) {
	t.Parallel()
	err := ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "gl",
		Name:                 "GitLab",
		Issuer:               "https://gitlab.example.com",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "gitlab",
		GitHub:               &GitHubOIDCProviderConfig{},
	}})
	if err == nil || !strings.Contains(err.Error(), "must not set github block") {
		t.Fatalf("got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "gl",
		Name:                 "GitLab",
		Issuer:               "https://gitlab.com",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "gitlab",
		GitLab: &GitLabOIDCProviderConfig{
			GroupRoleBindings: []GitLabGroupRoleBinding{{
				GroupFullPath: "a/b",
				Roles:         []string{},
			}},
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "roles must be non-empty") {
		t.Fatalf("got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "gl",
		Name:                 "GitLab",
		Issuer:               "https://gitlab.com",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "gitlab",
		GitLab: &GitLabOIDCProviderConfig{
			AllowedGroupPaths: []string{"acme"},
			GroupRoleBindings: []GitLabGroupRoleBinding{{
				GroupFullPath: "acme/devops",
				Roles:         []string{"api:contracts:export"},
			}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateOIDCProviders_NameRequired(t *testing.T) {
	t.Parallel()

	err := ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "github",
		Issuer:               GitHubOIDCDiscoveryIssuer,
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "github",
	}})
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("got %v", err)
	}
}

func TestValidateInteractiveTokenTTLs(t *testing.T) {
	t.Parallel()

	if err := ValidateInteractiveTokenTTLs(7*24*time.Hour, 30*24*time.Hour); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateInteractiveTokenTTLs(24*time.Hour, time.Hour); err == nil || !strings.Contains(err.Error(), "interactive_refresh_token_ttl") {
		t.Fatalf("expected ttl validation error, got %v", err)
	}
	if err := ValidateInteractiveTokenTTLs(0, 0); err != nil {
		t.Fatalf("zero values should resolve to defaults, got %v", err)
	}
}

func TestGitLabAPIV4BaseFromIssuer(t *testing.T) {
	t.Parallel()
	got, err := GitLabAPIV4BaseFromIssuer("https://gitlab.com/")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://gitlab.com/api/v4" {
		t.Fatalf("got %q", got)
	}
}

func TestValidateOIDCProviders_Google(t *testing.T) {
	t.Parallel()
	err := ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "g",
		Name:                 "Google",
		Issuer:               "https://accounts.google.com",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "google",
		GitHub:               &GitHubOIDCProviderConfig{},
	}})
	if err == nil || !strings.Contains(err.Error(), "must not set github block") {
		t.Fatalf("got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "g",
		Name:                 "Google",
		Issuer:               "https://evil.example",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "google",
	}})
	if err == nil || !strings.Contains(err.Error(), "kind=google requires issuer") {
		t.Fatalf("got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "g",
		Name:                 "Google",
		Issuer:               "https://accounts.google.com",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "google",
		Google: &GoogleOIDCProviderConfig{
			HostedDomainRoleBindings: []GoogleHostedDomainRoleBinding{{
				HD:    "example.com",
				Roles: []string{},
			}},
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "roles must be non-empty") {
		t.Fatalf("got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "g",
		Name:                 "Google",
		Issuer:               "https://accounts.google.com",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "google",
		Google: &GoogleOIDCProviderConfig{
			AllowedHostedDomains: []string{"example.com"},
			EmailDomainRoleBindings: []GoogleEmailDomainRoleBinding{{
				Domain: "example.com",
				Roles:  []string{"api:admin"},
			}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateOIDCProviders_Okta(t *testing.T) {
	t.Parallel()
	err := ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "ok",
		Name:                 "Okta",
		Issuer:               "https://dev-123.okta.com/oauth2/default",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "okta",
		GitHub:               &GitHubOIDCProviderConfig{},
	}})
	if err == nil || !strings.Contains(err.Error(), "must not set github block") {
		t.Fatalf("got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "ok",
		Name:                 "Okta",
		Issuer:               "http://dev-123.okta.com/oauth2/default",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "okta",
	}})
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "ok",
		Name:                 "Okta",
		Issuer:               "https://dev-123.okta.com/oauth2/default",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "okta",
		Okta: &OktaOIDCProviderConfig{
			GroupRoleBindings: []OktaGroupRoleBinding{{
				GroupName: "Everyone",
				Roles:     []string{},
			}},
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "roles must be non-empty") {
		t.Fatalf("got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "ok",
		Name:                 "Okta",
		Issuer:               "https://dev-123.okta.com/oauth2/default",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "okta",
		Okta: &OktaOIDCProviderConfig{
			AllowedIDTokenGroups: []string{"API-Admins"},
			GroupRoleBindings: []OktaGroupRoleBinding{{
				GroupName: "API-Admins",
				Roles:     []string{"api:admin"},
			}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateOIDCProviders_Entra(t *testing.T) {
	t.Parallel()
	err := ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "ent",
		Name:                 "Microsoft Entra ID",
		Issuer:               "https://login.microsoftonline.com/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/v2.0",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "entra",
		GitHub:               &GitHubOIDCProviderConfig{},
	}})
	if err == nil || !strings.Contains(err.Error(), "must not set github block") {
		t.Fatalf("got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "ent",
		Name:                 "Microsoft Entra ID",
		Issuer:               "http://login.microsoftonline.com/common/v2.0",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "entra",
	}})
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "ent",
		Name:                 "Microsoft Entra ID",
		Issuer:               "https://login.microsoftonline.com/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/v2.0",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "entra",
		Entra: &EntraOIDCProviderConfig{
			GroupRoleBindings: []EntraGroupRoleBinding{{
				Group: "g",
				Roles: []string{},
			}},
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "roles must be non-empty") {
		t.Fatalf("got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "ent",
		Name:                 "Microsoft Entra ID",
		Issuer:               "https://login.microsoftonline.com/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/v2.0",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "entra",
		Entra: &EntraOIDCProviderConfig{
			AllowedTenantIDs:     []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
			AllowedIDTokenGroups: []string{"admins"},
			GroupRoleBindings: []EntraGroupRoleBinding{{
				Group: "admins",
				Roles: []string{"api:admin"},
			}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateOIDCProviders_TwoGitHubKindDistinctIDs(t *testing.T) {
	t.Parallel()
	err := ValidateOIDCProviders([]OIDCProviderConfig{
		{
			ID:                   "github-corp-a",
			Name:                 "GitHub Corp A",
			Issuer:               GitHubOIDCDiscoveryIssuer,
			ClientID:             "a",
			ClientSecret:         "sa",
			RedirectURIAllowlist: []string{"http://127.0.0.1:21987/callback"},
			Kind:                 "github",
			GitHub:               &GitHubOIDCProviderConfig{AllowedOrgLogins: []string{"corp-a"}},
		},
		{
			ID:                   "github-corp-b",
			Name:                 "GitHub Corp B",
			Issuer:               GitHubOIDCDiscoveryIssuer,
			ClientID:             "b",
			ClientSecret:         "sb",
			RedirectURIAllowlist: []string{"http://127.0.0.1:21988/callback"},
			Kind:                 "github",
			GitHub:               &GitHubOIDCProviderConfig{AllowedOrgLogins: []string{"corp-b"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateOIDCProviders_GenericMustNotSetIdpBlocks(t *testing.T) {
	t.Parallel()
	err := ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "x",
		Name:                 "Generic OIDC",
		Issuer:               "https://idp.example",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		GitLab:               &GitLabOIDCProviderConfig{},
	}})
	if err == nil || !strings.Contains(err.Error(), "set kind: gitlab") {
		t.Fatalf("got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "x",
		Name:                 "Generic OIDC",
		Issuer:               "https://accounts.google.com",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Google:               &GoogleOIDCProviderConfig{},
	}})
	if err == nil || !strings.Contains(err.Error(), "set kind: google") {
		t.Fatalf("got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "x",
		Name:                 "Generic OIDC",
		Issuer:               "https://dev-1.okta.com/oauth2/default",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Okta:                 &OktaOIDCProviderConfig{},
	}})
	if err == nil || !strings.Contains(err.Error(), "set kind: okta") {
		t.Fatalf("got %v", err)
	}

	err = ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "x",
		Name:                 "Generic OIDC",
		Issuer:               "https://login.microsoftonline.com/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/v2.0",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Entra:                &EntraOIDCProviderConfig{},
	}})
	if err == nil || !strings.Contains(err.Error(), "set kind: entra") {
		t.Fatalf("got %v", err)
	}
}

func TestAuthConfig_BootstrapAPIKeyAllowed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		cfg   AuthConfig
		allow bool
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
