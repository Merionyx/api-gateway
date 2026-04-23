package config

import (
	"fmt"
	"strings"
	"time"
)

// OIDCProviderConfig describes one generic OIDC IdP for browser login (roadmap ш. 12–13).
type OIDCProviderConfig struct {
	// ID matches the login query parameter provider_id (opaque string, not a path segment).
	ID string `mapstructure:"id" json:"id"`
	// Issuer is the OIDC issuer base URL (no trailing slash); used for discovery.
	Issuer string `mapstructure:"issuer" json:"issuer"`
	// ClientID is the OAuth client_id sent to the authorization server.
	ClientID string `mapstructure:"client_id" json:"client_id"`
	// ClientSecret is the OAuth client_secret for the token endpoint (confidential client). Prefer env/K8s secret in prod.
	ClientSecret string `mapstructure:"client_secret" json:"client_secret"`
	// RedirectURIAllowlist is the exact set of allowed redirect_uri values for this provider.
	RedirectURIAllowlist []string `mapstructure:"redirect_uri_allowlist" json:"redirect_uri_allowlist"`
	// ExtraScopes are appended after "openid" (e.g. "email", "profile").
	ExtraScopes []string `mapstructure:"extra_scopes" json:"extra_scopes"`
	// Kind selects IdP-specific behavior. Empty or "generic" keeps standard OIDC only. "github" enables org/team checks (roadmap ш. 35).
	Kind string `mapstructure:"kind" json:"kind,omitempty"`
	// GitHub is read when Kind is "github" (allowed orgs, team→role bindings). Optional; nil means no extra restrictions beyond GitHub OAuth.
	GitHub *GitHubOIDCProviderConfig `mapstructure:"github" json:"github,omitempty"`
}

// GitHubOIDCProviderConfig restricts or enriches interactive login via GitHub REST (orgs, teams).
type GitHubOIDCProviderConfig struct {
	// RESTAPIBase overrides https://api.github.com (GitHub Enterprise Server: https://HOST/api/v3).
	RESTAPIBase string `mapstructure:"rest_api_base" json:"rest_api_base,omitempty"`
	// AllowedOrgLogins, if non-empty, requires the user to be a member of at least one listed organization (login names, case-insensitive).
	AllowedOrgLogins []string `mapstructure:"allowed_org_logins" json:"allowed_org_logins,omitempty"`
	// TeamRoleBindings grant API roles when the user belongs to org/team_slug on GitHub (requires read:org; added automatically for kind=github).
	TeamRoleBindings []GitHubTeamRoleBinding `mapstructure:"team_role_bindings" json:"team_role_bindings,omitempty"`
}

// GitHubTeamRoleBinding maps GitHub org membership in a team to API Server role strings.
type GitHubTeamRoleBinding struct {
	Org      string   `mapstructure:"org" json:"org"`
	TeamSlug string   `mapstructure:"team_slug" json:"team_slug"`
	Roles    []string `mapstructure:"roles" json:"roles,omitempty"`
}

// GitHubOIDCDiscoveryIssuer is the issuer URL used in FetchDiscovery for GitHub browser OAuth (openid-configuration lives under this path).
const GitHubOIDCDiscoveryIssuer = "https://github.com/login/oauth"

// IsGitHubOIDCProvider reports whether this entry uses GitHub-specific org/team handling.
func (p OIDCProviderConfig) IsGitHubOIDCProvider() bool {
	return strings.EqualFold(strings.TrimSpace(p.Kind), "github")
}

// AuthConfig controls auth-related etcd keys and dev-only bootstrap (roadmap п. 20).
type AuthConfig struct {
	// EtcdKeyPrefix is the auth root (default /api-gateway/api-server/auth/v1). Trailing slashes are ignored.
	EtcdKeyPrefix string `mapstructure:"etcd_key_prefix" json:"etcd_key_prefix"`

	// Environment is a coarse deployment class: "development", "local", "production", etc.
	// Insecure API key bootstrap is allowed only when AllowInsecureBootstrap is true AND
	// Environment is "development" or "local" (never "production").
	Environment string `mapstructure:"environment" json:"environment"`

	// AllowInsecureBootstrap enables POST-less bootstrap of the first API key record into etcd
	// from application code when combined with a non-production Environment (see BootstrapAPIKeyAllowed).
	// Must remain false in production Helm values.
	AllowInsecureBootstrap bool `mapstructure:"allow_insecure_bootstrap" json:"allow_insecure_bootstrap"`

	// OIDCProviders configures browser OIDC login (GET /api/v1/auth/login). Empty disables login until configured.
	OIDCProviders []OIDCProviderConfig `mapstructure:"oidc_providers" json:"oidc_providers"`

	// LoginIntentLeaseTTL is the etcd lease for login-intent keys (short-lived; default 15m).
	LoginIntentLeaseTTL time.Duration `mapstructure:"login_intent_lease_ttl" json:"login_intent_lease_ttl"`

	// SessionKEKBase64 is standard base64 of 32 bytes (AES-256) used to seal IdP refresh material in session values.
	// Required when oidc_providers is non-empty (roadmap ш. 8–11, ш. 14).
	SessionKEKBase64 string `mapstructure:"session_kek_base64" json:"session_kek_base64"`

	// InteractiveAccessTokenTTL is our API-profile access JWT lifetime after OIDC login (default 5m; roadmap).
	InteractiveAccessTokenTTL time.Duration `mapstructure:"interactive_access_token_ttl" json:"interactive_access_token_ttl"`

	// IdpAccessCacheOpaqueMaxTTL caps inferred IdP access lifetime for opaque tokens without expires_in/JWT exp (ADR 0002, roadmap ш. 19). Zero uses idpcache.DefaultOpaqueMaxTTL for that branch only.
	IdpAccessCacheOpaqueMaxTTL time.Duration `mapstructure:"idp_access_cache_opaque_max_ttl" json:"idp_access_cache_opaque_max_ttl"`
}

// ValidateOIDCProviders returns an error if the slice is inconsistent (duplicate id, missing fields, empty allowlist).
func ValidateOIDCProviders(providers []OIDCProviderConfig) error {
	seen := make(map[string]struct{}, len(providers))
	for i, p := range providers {
		id := strings.TrimSpace(p.ID)
		if id == "" {
			return fmt.Errorf("auth.oidc_providers[%d]: id is required", i)
		}
		if _, dup := seen[id]; dup {
			return fmt.Errorf("auth: duplicate oidc provider id %q", id)
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(p.Issuer) == "" {
			return fmt.Errorf("auth.oidc_providers[%q]: issuer is required", id)
		}
		if strings.TrimSpace(p.ClientID) == "" {
			return fmt.Errorf("auth.oidc_providers[%q]: client_id is required", id)
		}
		if len(p.RedirectURIAllowlist) == 0 {
			return fmt.Errorf("auth.oidc_providers[%q]: redirect_uri_allowlist must be non-empty", id)
		}
		if err := validateOIDCProviderKind(id, p); err != nil {
			return err
		}
	}
	return nil
}

func validateOIDCProviderKind(id string, p OIDCProviderConfig) error {
	k := strings.ToLower(strings.TrimSpace(p.Kind))
	switch k {
	case "", "generic":
		return nil
	case "github":
		if strings.TrimSuffix(strings.TrimSpace(p.Issuer), "/") != GitHubOIDCDiscoveryIssuer {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=github requires issuer %q (documented id_token iss is still https://github.com)", id, GitHubOIDCDiscoveryIssuer)
		}
		g := p.GitHub
		if g == nil {
			return nil
		}
		for j, org := range g.AllowedOrgLogins {
			if strings.TrimSpace(org) == "" {
				return fmt.Errorf("auth.oidc_providers[%q].github.allowed_org_logins[%d]: empty entry", id, j)
			}
		}
		for j, b := range g.TeamRoleBindings {
			if strings.TrimSpace(b.Org) == "" {
				return fmt.Errorf("auth.oidc_providers[%q].github.team_role_bindings[%d]: org is required", id, j)
			}
			if strings.TrimSpace(b.TeamSlug) == "" {
				return fmt.Errorf("auth.oidc_providers[%q].github.team_role_bindings[%d]: team_slug is required", id, j)
			}
			if len(b.Roles) == 0 {
				return fmt.Errorf("auth.oidc_providers[%q].github.team_role_bindings[%d]: roles must be non-empty", id, j)
			}
			for ri, role := range b.Roles {
				if strings.TrimSpace(role) == "" {
					return fmt.Errorf("auth.oidc_providers[%q].github.team_role_bindings[%d].roles[%d]: empty role", id, j, ri)
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("auth.oidc_providers[%q]: unknown kind %q (supported: generic, github)", id, strings.TrimSpace(p.Kind))
	}
}

// BootstrapAPIKeyAllowed reports whether the insecure bootstrap path may run (roadmap step 10).
func (a AuthConfig) BootstrapAPIKeyAllowed() bool {
	if !a.AllowInsecureBootstrap {
		return false
	}
	e := strings.ToLower(strings.TrimSpace(a.Environment))
	return e == "development" || e == "local"
}
