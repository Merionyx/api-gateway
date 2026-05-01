package config

import (
	"strings"
	"testing"
	"time"
)

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

func TestValidateOIDCProviders_GitHub_AuthFlowAndIssuer(t *testing.T) {
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

func TestValidateOIDCProviders_RejectsLegacyRoleMappingFields(t *testing.T) {
	t.Parallel()
	cases := []OIDCProviderConfig{
		{
			ID:                   "gh",
			Name:                 "GitHub",
			Issuer:               GitHubOIDCDiscoveryIssuer,
			ClientID:             "c",
			ClientSecret:         "s",
			RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
			Kind:                 "github",
			GitHub: &GitHubOIDCProviderConfig{
				AllowedOrgLogins: []string{"acme"},
			},
		},
		{
			ID:                   "gl",
			Name:                 "GitLab",
			Issuer:               "https://gitlab.com",
			ClientID:             "c",
			ClientSecret:         "s",
			RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
			Kind:                 "gitlab",
			GitLab: &GitLabOIDCProviderConfig{
				AllowedGroupPaths: []string{"acme"},
			},
		},
		{
			ID:                   "google",
			Name:                 "Google",
			Issuer:               GoogleOIDCDiscoveryIssuer,
			ClientID:             "c",
			ClientSecret:         "s",
			RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
			Kind:                 "google",
			Google: &GoogleOIDCProviderConfig{
				AllowedHostedDomains: []string{"example.com"},
			},
		},
		{
			ID:                   "okta",
			Name:                 "Okta",
			Issuer:               "https://dev-123.okta.com/oauth2/default",
			ClientID:             "c",
			ClientSecret:         "s",
			RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
			Kind:                 "okta",
			Okta: &OktaOIDCProviderConfig{
				AllowedIDTokenGroups: []string{"admins"},
			},
		},
		{
			ID:                   "entra",
			Name:                 "Entra",
			Issuer:               "https://login.microsoftonline.com/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa/v2.0",
			ClientID:             "c",
			ClientSecret:         "s",
			RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
			Kind:                 "entra",
			Entra: &EntraOIDCProviderConfig{
				AllowedTenantIDs: []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},
			},
		},
	}
	for i := range cases {
		err := ValidateOIDCProviders([]OIDCProviderConfig{cases[i]})
		if err == nil || !strings.Contains(err.Error(), "migrate to claim_mapping.rules") {
			t.Fatalf("case[%d] expected migration error, got %v", i, err)
		}
	}
}

func TestValidateOIDCProviders_ClaimMapping(t *testing.T) {
	t.Parallel()
	base := OIDCProviderConfig{
		ID:                   "gh",
		Name:                 "GitHub",
		Issuer:               GitHubOIDCDiscoveryIssuer,
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		Kind:                 "github",
	}

	badMissingWhen := base
	badMissingWhen.ClaimMapping = &OIDCClaimMappingConfig{Rules: []OIDCClaimMappingRule{{
		AddRoles: []string{"api:role:admin"},
	}}}
	err := ValidateOIDCProviders([]OIDCProviderConfig{badMissingWhen})
	if err == nil || !strings.Contains(err.Error(), "when is required") {
		t.Fatalf("got %v", err)
	}

	badNoAction := base
	badNoAction.ClaimMapping = &OIDCClaimMappingConfig{Rules: []OIDCClaimMappingRule{{
		When: "true",
	}}}
	err = ValidateOIDCProviders([]OIDCProviderConfig{badNoAction})
	if err == nil || !strings.Contains(err.Error(), "rule has no action") {
		t.Fatalf("got %v", err)
	}

	badReservedClaim := base
	badReservedClaim.ClaimMapping = &OIDCClaimMappingConfig{Rules: []OIDCClaimMappingRule{{
		When:      "true",
		SetClaims: map[string]string{"iss": "\"x\""},
	}}}
	err = ValidateOIDCProviders([]OIDCProviderConfig{badReservedClaim})
	if err == nil || !strings.Contains(err.Error(), "reserved JWT claim") {
		t.Fatalf("got %v", err)
	}

	badDenyWithAction := base
	badDenyWithAction.ClaimMapping = &OIDCClaimMappingConfig{Rules: []OIDCClaimMappingRule{{
		When:     "true",
		Deny:     true,
		AddRoles: []string{"api:role:admin"},
	}}}
	err = ValidateOIDCProviders([]OIDCProviderConfig{badDenyWithAction})
	if err == nil || !strings.Contains(err.Error(), "deny rule cannot include") {
		t.Fatalf("got %v", err)
	}

	good := base
	good.ClaimMapping = &OIDCClaimMappingConfig{Rules: []OIDCClaimMappingRule{{
		Name:           "admins",
		When:           "provider.kind == 'github'",
		AddRoles:       []string{"api:role:admin"},
		AddPermissions: []string{"api.contracts.export"},
		SetClaims: map[string]string{
			"team": "'platform'",
		},
	}}}
	err = ValidateOIDCProviders([]OIDCProviderConfig{good})
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateOIDCProviders_TwoGitHubKindDistinctIDs(t *testing.T) {
	t.Parallel()
	err := ValidateOIDCProviders([]OIDCProviderConfig{
		{
			ID:                   "github-a",
			Name:                 "GitHub A",
			Issuer:               GitHubOIDCDiscoveryIssuer,
			ClientID:             "a",
			ClientSecret:         "sa",
			RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
			Kind:                 "github",
		},
		{
			ID:                   "github-b",
			Name:                 "GitHub B",
			Issuer:               GitHubOIDCDiscoveryIssuer,
			ClientID:             "b",
			ClientSecret:         "sb",
			RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
			Kind:                 "github",
		},
	})
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateOIDCProviders_GenericMustNotSetIdpBlocks(t *testing.T) {
	t.Parallel()
	err := ValidateOIDCProviders([]OIDCProviderConfig{{
		ID:                   "g",
		Name:                 "Generic",
		Issuer:               "https://idp.example",
		ClientID:             "c",
		ClientSecret:         "s",
		RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
		GitLab:               &GitLabOIDCProviderConfig{},
	}})
	if err == nil || !strings.Contains(err.Error(), "set kind: gitlab") {
		t.Fatalf("got %v", err)
	}
}

func TestOIDCCallbackURIFromExternalBase(t *testing.T) {
	t.Parallel()

	got, err := OIDCCallbackURIFromExternalBase("https://api.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://api.example.com/v1/auth/callback" {
		t.Fatalf("got %q", got)
	}

	got, err = OIDCCallbackURIFromExternalBase("http://127.0.0.1:8080/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://127.0.0.1:8080/v1/auth/callback" {
		t.Fatalf("got %q", got)
	}
}

func TestOIDCCallbackURIFromExternalBase_Invalid(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"api.example.com",
		"https://user:pass@api.example.com",
		"https://api.example.com/path",
		"https://api.example.com?x=1",
		"ftp://api.example.com",
	}

	for _, in := range cases {
		in := in
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			if _, err := OIDCCallbackURIFromExternalBase(in); err == nil {
				t.Fatalf("expected error for %q", in)
			}
		})
	}
}

func TestValidateOIDCExternalBaseURL(t *testing.T) {
	t.Parallel()

	providers := []OIDCProviderConfig{{
		ID:                   "idp",
		Name:                 "IDP",
		Issuer:               "https://idp.example.com",
		ClientID:             "client",
		ClientSecret:         "secret",
		RedirectURIAllowlist: []string{"http://127.0.0.1/callback"},
	}}

	if err := ValidateOIDCExternalBaseURL("", nil); err != nil {
		t.Fatalf("empty providers should not require base URL: %v", err)
	}
	if err := ValidateOIDCExternalBaseURL("", providers); err == nil {
		t.Fatal("expected error when providers are configured and external base URL is empty")
	}
	if err := ValidateOIDCExternalBaseURL("https://api.example.com", providers); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateInteractiveTokenTTLPolicy(t *testing.T) {
	t.Parallel()

	if err := ValidateInteractiveTokenTTLPolicy(7*24*time.Hour, 30*24*time.Hour, 30*24*time.Hour, 90*24*time.Hour); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateInteractiveTokenTTLPolicy(24*time.Hour, 30*24*time.Hour, time.Hour, 90*24*time.Hour); err == nil || !strings.Contains(err.Error(), "interactive_refresh_token_ttl") {
		t.Fatalf("expected ttl validation error, got %v", err)
	}
	if err := ValidateInteractiveTokenTTLPolicy(0, 0, 0, 0); err != nil {
		t.Fatalf("zero values should resolve to defaults, got %v", err)
	}
	if err := ValidateInteractiveTokenTTLPolicy(7*24*time.Hour, time.Hour, 30*24*time.Hour, 90*24*time.Hour); err == nil || !strings.Contains(err.Error(), "interactive_access_token_max_ttl") {
		t.Fatalf("expected access max validation error, got %v", err)
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
