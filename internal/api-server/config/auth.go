package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	// DefaultInteractiveAccessTokenTTL is the default API-profile access JWT lifetime after OIDC login.
	DefaultInteractiveAccessTokenTTL = 5 * time.Minute
	// DefaultInteractiveRefreshTokenTTL is the default maximum lifetime of our interactive refresh chain.
	DefaultInteractiveRefreshTokenTTL = 7 * 24 * time.Hour
)

// OIDCProviderConfig describes one generic OIDC IdP for browser login (roadmap ш. 12–13).
type OIDCProviderConfig struct {
	// ID matches the login query parameter provider_id (opaque string, not a path segment).
	ID string `mapstructure:"id" json:"id"`
	// Name is the user-facing provider label shown in APIs and CLI (e.g. "GitHub", "GitHub Enterprise").
	Name string `mapstructure:"name" json:"name"`
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
	// Kind selects IdP-specific behavior. Empty or "generic" keeps standard OIDC only. "github" enables org/team checks (roadmap ш. 35). "gitlab" enables group checks (ш. 36). "google" uses id_token hd/email (ш. 37). "okta" uses id_token groups (ш. 38). "entra" uses id_token tid/groups (ш. 39).
	Kind string `mapstructure:"kind" json:"kind,omitempty"`
	// GitHub is read when Kind is "github" (allowed orgs, team→role bindings). Optional; nil means no extra restrictions beyond GitHub OAuth.
	GitHub *GitHubOIDCProviderConfig `mapstructure:"github" json:"github,omitempty"`
	// GitLab is read when Kind is "gitlab" (allowed group paths, group→role bindings).
	GitLab *GitLabOIDCProviderConfig `mapstructure:"gitlab" json:"gitlab,omitempty"`
	// Google is read when Kind is "google" (Workspace hd / email domain allowlist and bindings from id_token claims).
	Google *GoogleOIDCProviderConfig `mapstructure:"google" json:"google,omitempty"`
	// Okta is read when Kind is "okta" (id_token "groups" claim allowlist and bindings; configure Okta to emit groups).
	Okta *OktaOIDCProviderConfig `mapstructure:"okta" json:"okta,omitempty"`
	// Entra is read when Kind is "entra" (Microsoft Entra ID: id_token tid + groups claim; configure app for group claims).
	Entra *EntraOIDCProviderConfig `mapstructure:"entra" json:"entra,omitempty"`
}

// GitHubOIDCProviderConfig restricts or enriches interactive login via GitHub REST (orgs, teams).
type GitHubOIDCProviderConfig struct {
	// AuthFlow selects how browser authorization is initiated.
	// "oauth_app" (default) keeps OAuth App style scopes like read:org.
	// "github_app" uses GitHub App user authorization flow, which does not send OAuth scopes.
	AuthFlow string `mapstructure:"auth_flow" json:"auth_flow,omitempty"`
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

// GitLabOIDCProviderConfig restricts or enriches interactive login via GitLab REST API v4 (groups).
type GitLabOIDCProviderConfig struct {
	// APIV4Base overrides the default {issuer origin}/api/v4 (e.g. https://gitlab.com/api/v4 or self-managed).
	APIV4Base string `mapstructure:"api_v4_base" json:"api_v4_base,omitempty"`
	// AllowedGroupPaths, if non-empty, requires the user to belong to at least one group whose full_path equals a path
	// or starts with path+"/" (GitLab group path, case-sensitive as on GitLab).
	AllowedGroupPaths []string `mapstructure:"allowed_group_paths" json:"allowed_group_paths,omitempty"`
	// GroupRoleBindings grant API roles when the user is a direct member of the given group full_path.
	GroupRoleBindings []GitLabGroupRoleBinding `mapstructure:"group_role_bindings" json:"group_role_bindings,omitempty"`
}

// GitLabGroupRoleBinding maps membership in a GitLab group (full_path) to API Server role strings.
type GitLabGroupRoleBinding struct {
	GroupFullPath string   `mapstructure:"group_full_path" json:"group_full_path"`
	Roles         []string `mapstructure:"roles" json:"roles,omitempty"`
}

// IsGitHubOIDCProvider reports whether this entry uses GitHub-specific org/team handling.
func (p OIDCProviderConfig) IsGitHubOIDCProvider() bool {
	return strings.EqualFold(strings.TrimSpace(p.Kind), "github")
}

// GitHubAuthFlow returns the normalized GitHub browser auth flow mode.
func (p OIDCProviderConfig) GitHubAuthFlow() string {
	if p.GitHub == nil {
		return "oauth_app"
	}
	switch strings.ToLower(strings.TrimSpace(p.GitHub.AuthFlow)) {
	case "", "oauth_app":
		return "oauth_app"
	case "github_app":
		return "github_app"
	default:
		return strings.ToLower(strings.TrimSpace(p.GitHub.AuthFlow))
	}
}

// IsGitLabOIDCProvider reports whether this entry uses GitLab-specific group handling.
func (p OIDCProviderConfig) IsGitLabOIDCProvider() bool {
	return strings.EqualFold(strings.TrimSpace(p.Kind), "gitlab")
}

// GoogleOIDCProviderConfig restricts or enriches login using Google id_token claims (hd, email); no Admin SDK in MVP.
type GoogleOIDCProviderConfig struct {
	// AllowedHostedDomains, if non-empty, requires a non-empty id_token "hd" claim matching one entry (Google Workspace; case-insensitive).
	AllowedHostedDomains []string `mapstructure:"allowed_hosted_domains" json:"allowed_hosted_domains,omitempty"`
	// AllowedEmailDomains, if used without allowed_hosted_domains, requires email domain to match one entry (case-insensitive, no leading @ in config).
	AllowedEmailDomains []string `mapstructure:"allowed_email_domains" json:"allowed_email_domains,omitempty"`
	// HostedDomainRoleBindings grant roles when id_token "hd" matches (case-insensitive).
	HostedDomainRoleBindings []GoogleHostedDomainRoleBinding `mapstructure:"hosted_domain_role_bindings" json:"hosted_domain_role_bindings,omitempty"`
	// EmailDomainRoleBindings grant roles when the email address domain matches (case-insensitive).
	EmailDomainRoleBindings []GoogleEmailDomainRoleBinding `mapstructure:"email_domain_role_bindings" json:"email_domain_role_bindings,omitempty"`
}

// GoogleHostedDomainRoleBinding maps a Workspace hosted domain (hd claim) to API roles.
type GoogleHostedDomainRoleBinding struct {
	HD    string   `mapstructure:"hd" json:"hd"`
	Roles []string `mapstructure:"roles" json:"roles,omitempty"`
}

// GoogleEmailDomainRoleBinding maps an email domain suffix to API roles.
type GoogleEmailDomainRoleBinding struct {
	Domain string   `mapstructure:"domain" json:"domain"`
	Roles  []string `mapstructure:"roles" json:"roles,omitempty"`
}

// GoogleOIDCDiscoveryIssuer is the issuer for Google accounts (browser OIDC).
const GoogleOIDCDiscoveryIssuer = "https://accounts.google.com"

// IsGoogleOIDCProvider reports whether this entry uses Google-specific claim handling.
func (p OIDCProviderConfig) IsGoogleOIDCProvider() bool {
	return strings.EqualFold(strings.TrimSpace(p.Kind), "google")
}

// OktaOIDCProviderConfig restricts or enriches login using the id_token "groups" claim (configure Okta Authorization Server + app).
type OktaOIDCProviderConfig struct {
	// AllowedIDTokenGroups, if non-empty, requires the id_token "groups" claim to contain at least one of these names (exact match after TrimSpace).
	AllowedIDTokenGroups []string `mapstructure:"allowed_id_token_groups" json:"allowed_id_token_groups,omitempty"`
	// GroupRoleBindings grant API roles when the user has the given group name in id_token "groups".
	GroupRoleBindings []OktaGroupRoleBinding `mapstructure:"group_role_bindings" json:"group_role_bindings,omitempty"`
}

// OktaGroupRoleBinding maps an Okta group name (as in id_token) to API Server role strings.
type OktaGroupRoleBinding struct {
	GroupName string   `mapstructure:"group_name" json:"group_name"`
	Roles     []string `mapstructure:"roles" json:"roles,omitempty"`
}

// IsOktaOIDCProvider reports whether this entry uses Okta-specific id_token groups handling.
func (p OIDCProviderConfig) IsOktaOIDCProvider() bool {
	return strings.EqualFold(strings.TrimSpace(p.Kind), "okta")
}

// EntraOIDCProviderConfig restricts or enriches login using Microsoft Entra id_token claims (tid, groups).
type EntraOIDCProviderConfig struct {
	// AllowedTenantIDs, if non-empty, requires id_token "tid" to match one entry (GUID string; comparison is case-insensitive).
	AllowedTenantIDs []string `mapstructure:"allowed_tenant_ids" json:"allowed_tenant_ids,omitempty"`
	// AllowedIDTokenGroups, if non-empty, requires id_token "groups" to intersect this list (exact string match after TrimSpace, as emitted in the token).
	AllowedIDTokenGroups []string `mapstructure:"allowed_id_token_groups" json:"allowed_id_token_groups,omitempty"`
	// GroupRoleBindings grant roles when an id_token "groups" entry matches Group (object ID or name, depending on Entra token configuration).
	GroupRoleBindings []EntraGroupRoleBinding `mapstructure:"group_role_bindings" json:"group_role_bindings,omitempty"`
}

// EntraGroupRoleBinding maps a group entry from the id_token "groups" claim to API Server role strings.
type EntraGroupRoleBinding struct {
	Group string   `mapstructure:"group" json:"group"`
	Roles []string `mapstructure:"roles" json:"roles,omitempty"`
}

// IsEntraOIDCProvider reports whether this entry uses Microsoft Entra-specific id_token handling.
func (p OIDCProviderConfig) IsEntraOIDCProvider() bool {
	return strings.EqualFold(strings.TrimSpace(p.Kind), "entra")
}

// ValidateHTTPSOIDCIssuer requires a parsed https URL with a host (Okta, Entra v2.0 issuer, etc.).
func ValidateHTTPSOIDCIssuer(issuer string) error {
	u, err := url.Parse(strings.TrimSpace(issuer))
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid issuer URL")
	}
	if strings.ToLower(u.Scheme) != "https" {
		return fmt.Errorf("issuer must use https")
	}
	return nil
}

// ValidateOktaIssuer returns an error if issuer is not a usable https URL for Okta OIDC (Authorization Server issuer).
func ValidateOktaIssuer(issuer string) error {
	return ValidateHTTPSOIDCIssuer(issuer)
}

// ValidateEntraIssuer returns an error if issuer is not a usable https URL for Entra OIDC (v2.0 issuer).
func ValidateEntraIssuer(issuer string) error {
	return ValidateHTTPSOIDCIssuer(issuer)
}

// GitLabAPIV4BaseFromIssuer returns {scheme}://{host}/api/v4 for a GitLab OIDC issuer (no path after host).
func GitLabAPIV4BaseFromIssuer(issuer string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(issuer))
	if err != nil || u.Host == "" || u.Scheme == "" {
		return "", fmt.Errorf("invalid gitlab issuer URL")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("gitlab issuer must use http or https")
	}
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimSuffix(u.String(), "/") + "/api/v4", nil
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

	// InteractiveAccessTokenTTL is the default API-profile access JWT lifetime after OIDC login (default 5m; roadmap).
	InteractiveAccessTokenTTL time.Duration `mapstructure:"interactive_access_token_ttl" json:"interactive_access_token_ttl"`

	// InteractiveAccessTokenMaxTTL is the maximum API-profile access JWT lifetime a client may request.
	InteractiveAccessTokenMaxTTL time.Duration `mapstructure:"interactive_access_token_max_ttl" json:"interactive_access_token_max_ttl"`

	// InteractiveRefreshTokenTTL is the default lifetime of our interactive refresh chain (default 7d).
	InteractiveRefreshTokenTTL time.Duration `mapstructure:"interactive_refresh_token_ttl" json:"interactive_refresh_token_ttl"`

	// InteractiveRefreshTokenMaxTTL is the maximum lifetime a client may request for our refresh chain.
	// When the IdP discloses a shorter refresh lifetime, our session is clamped to that shorter deadline.
	InteractiveRefreshTokenMaxTTL time.Duration `mapstructure:"interactive_refresh_token_max_ttl" json:"interactive_refresh_token_max_ttl"`

	// IdpAccessCacheOpaqueMaxTTL caps inferred IdP access lifetime for opaque tokens without expires_in/JWT exp (ADR 0002, roadmap ш. 19). Zero uses idpcache.DefaultOpaqueMaxTTL for that branch only.
	IdpAccessCacheOpaqueMaxTTL time.Duration `mapstructure:"idp_access_cache_opaque_max_ttl" json:"idp_access_cache_opaque_max_ttl"`
}

func EffectiveInteractiveAccessTokenTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return DefaultInteractiveAccessTokenTTL
	}
	return ttl
}

func EffectiveInteractiveRefreshTokenTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return DefaultInteractiveRefreshTokenTTL
	}
	return ttl
}

func EffectiveInteractiveAccessTokenMaxTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return DefaultInteractiveAccessTokenTTL
	}
	return ttl
}

func EffectiveInteractiveRefreshTokenMaxTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return DefaultInteractiveRefreshTokenTTL
	}
	return ttl
}

// ValidateInteractiveTokenTTLPolicy validates interactive default/max lifetimes after applying defaults for zero values.
func ValidateInteractiveTokenTTLPolicy(accessTTL, accessMaxTTL, refreshTTL, refreshMaxTTL time.Duration) error {
	accessTTL = EffectiveInteractiveAccessTokenTTL(accessTTL)
	accessMaxTTL = EffectiveInteractiveAccessTokenMaxTTL(accessMaxTTL)
	refreshTTL = EffectiveInteractiveRefreshTokenTTL(refreshTTL)
	refreshMaxTTL = EffectiveInteractiveRefreshTokenMaxTTL(refreshMaxTTL)
	if accessTTL <= 0 {
		return fmt.Errorf("auth.interactive_access_token_ttl must be > 0")
	}
	if accessMaxTTL <= 0 {
		return fmt.Errorf("auth.interactive_access_token_max_ttl must be > 0")
	}
	if refreshTTL <= 0 {
		return fmt.Errorf("auth.interactive_refresh_token_ttl must be > 0")
	}
	if refreshMaxTTL <= 0 {
		return fmt.Errorf("auth.interactive_refresh_token_max_ttl must be > 0")
	}
	if accessTTL > accessMaxTTL {
		return fmt.Errorf("auth.interactive_access_token_ttl (%s) must be <= auth.interactive_access_token_max_ttl (%s)", accessTTL, accessMaxTTL)
	}
	if refreshTTL > refreshMaxTTL {
		return fmt.Errorf("auth.interactive_refresh_token_ttl (%s) must be <= auth.interactive_refresh_token_max_ttl (%s)", refreshTTL, refreshMaxTTL)
	}
	if refreshTTL < accessTTL {
		return fmt.Errorf("auth.interactive_refresh_token_ttl (%s) must be >= auth.interactive_access_token_ttl (%s)", refreshTTL, accessTTL)
	}
	if refreshMaxTTL < accessMaxTTL {
		return fmt.Errorf("auth.interactive_refresh_token_max_ttl (%s) must be >= auth.interactive_access_token_max_ttl (%s)", refreshMaxTTL, accessMaxTTL)
	}
	return nil
}

// ValidateOIDCProviders returns an error if the slice is inconsistent (duplicate id, missing fields, empty allowlist).
func ValidateOIDCProviders(providers []OIDCProviderConfig) error {
	seen := make(map[string]struct{}, len(providers))
	for i, p := range providers {
		id := strings.TrimSpace(p.ID)
		if id == "" {
			return fmt.Errorf("auth.oidc_providers[%d]: id is required", i)
		}
		if strings.TrimSpace(p.Name) == "" {
			return fmt.Errorf("auth.oidc_providers[%q]: name is required", id)
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
		if p.GitHub != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: set kind: github when using github block", id)
		}
		if p.GitLab != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: set kind: gitlab when using gitlab block", id)
		}
		if p.Google != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: set kind: google when using google block", id)
		}
		if p.Okta != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: set kind: okta when using okta block", id)
		}
		if p.Entra != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: set kind: entra when using entra block", id)
		}
		return nil
	case "github":
		if p.GitLab != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=github must not set gitlab block", id)
		}
		if p.Google != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=github must not set google block", id)
		}
		if p.Okta != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=github must not set okta block", id)
		}
		if p.Entra != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=github must not set entra block", id)
		}
		if strings.TrimSuffix(strings.TrimSpace(p.Issuer), "/") != GitHubOIDCDiscoveryIssuer {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=github requires issuer %q (documented id_token iss is still https://github.com)", id, GitHubOIDCDiscoveryIssuer)
		}
		g := p.GitHub
		if flow := p.GitHubAuthFlow(); flow != "oauth_app" && flow != "github_app" {
			return fmt.Errorf("auth.oidc_providers[%q].github.auth_flow: want oauth_app or github_app", id)
		}
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
	case "gitlab":
		if p.GitHub != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=gitlab must not set github block", id)
		}
		if p.Google != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=gitlab must not set google block", id)
		}
		if p.Okta != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=gitlab must not set okta block", id)
		}
		if p.Entra != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=gitlab must not set entra block", id)
		}
		if _, err := GitLabAPIV4BaseFromIssuer(p.Issuer); err != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=gitlab requires a valid issuer URL (e.g. https://gitlab.com or self-managed origin): %w", id, err)
		}
		gl := p.GitLab
		if gl == nil {
			return nil
		}
		if b := strings.TrimSpace(gl.APIV4Base); b != "" {
			if _, err := url.Parse(b); err != nil || !strings.HasPrefix(strings.ToLower(b), "http") {
				return fmt.Errorf("auth.oidc_providers[%q].gitlab.api_v4_base: invalid URL", id)
			}
		}
		for j, path := range gl.AllowedGroupPaths {
			if strings.TrimSpace(path) == "" {
				return fmt.Errorf("auth.oidc_providers[%q].gitlab.allowed_group_paths[%d]: empty entry", id, j)
			}
		}
		for j, b := range gl.GroupRoleBindings {
			if strings.TrimSpace(b.GroupFullPath) == "" {
				return fmt.Errorf("auth.oidc_providers[%q].gitlab.group_role_bindings[%d]: group_full_path is required", id, j)
			}
			if len(b.Roles) == 0 {
				return fmt.Errorf("auth.oidc_providers[%q].gitlab.group_role_bindings[%d]: roles must be non-empty", id, j)
			}
			for ri, role := range b.Roles {
				if strings.TrimSpace(role) == "" {
					return fmt.Errorf("auth.oidc_providers[%q].gitlab.group_role_bindings[%d].roles[%d]: empty role", id, j, ri)
				}
			}
		}
		return nil
	case "google":
		if p.GitHub != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=google must not set github block", id)
		}
		if p.GitLab != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=google must not set gitlab block", id)
		}
		if p.Okta != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=google must not set okta block", id)
		}
		if p.Entra != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=google must not set entra block", id)
		}
		if strings.TrimSuffix(strings.TrimSpace(p.Issuer), "/") != GoogleOIDCDiscoveryIssuer {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=google requires issuer %q", id, GoogleOIDCDiscoveryIssuer)
		}
		g := p.Google
		if g == nil {
			return nil
		}
		for j, d := range g.AllowedHostedDomains {
			if strings.TrimSpace(d) == "" {
				return fmt.Errorf("auth.oidc_providers[%q].google.allowed_hosted_domains[%d]: empty entry", id, j)
			}
		}
		for j, d := range g.AllowedEmailDomains {
			if strings.TrimSpace(d) == "" {
				return fmt.Errorf("auth.oidc_providers[%q].google.allowed_email_domains[%d]: empty entry", id, j)
			}
		}
		for j, b := range g.HostedDomainRoleBindings {
			if strings.TrimSpace(b.HD) == "" {
				return fmt.Errorf("auth.oidc_providers[%q].google.hosted_domain_role_bindings[%d]: hd is required", id, j)
			}
			if len(b.Roles) == 0 {
				return fmt.Errorf("auth.oidc_providers[%q].google.hosted_domain_role_bindings[%d]: roles must be non-empty", id, j)
			}
			for ri, role := range b.Roles {
				if strings.TrimSpace(role) == "" {
					return fmt.Errorf("auth.oidc_providers[%q].google.hosted_domain_role_bindings[%d].roles[%d]: empty role", id, j, ri)
				}
			}
		}
		for j, b := range g.EmailDomainRoleBindings {
			if strings.TrimSpace(b.Domain) == "" {
				return fmt.Errorf("auth.oidc_providers[%q].google.email_domain_role_bindings[%d]: domain is required", id, j)
			}
			if len(b.Roles) == 0 {
				return fmt.Errorf("auth.oidc_providers[%q].google.email_domain_role_bindings[%d]: roles must be non-empty", id, j)
			}
			for ri, role := range b.Roles {
				if strings.TrimSpace(role) == "" {
					return fmt.Errorf("auth.oidc_providers[%q].google.email_domain_role_bindings[%d].roles[%d]: empty role", id, j, ri)
				}
			}
		}
		return nil
	case "okta":
		if p.GitHub != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=okta must not set github block", id)
		}
		if p.GitLab != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=okta must not set gitlab block", id)
		}
		if p.Google != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=okta must not set google block", id)
		}
		if p.Entra != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=okta must not set entra block", id)
		}
		if err := ValidateOktaIssuer(p.Issuer); err != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=okta: %w", id, err)
		}
		o := p.Okta
		if o == nil {
			return nil
		}
		for j, g := range o.AllowedIDTokenGroups {
			if strings.TrimSpace(g) == "" {
				return fmt.Errorf("auth.oidc_providers[%q].okta.allowed_id_token_groups[%d]: empty entry", id, j)
			}
		}
		for j, b := range o.GroupRoleBindings {
			if strings.TrimSpace(b.GroupName) == "" {
				return fmt.Errorf("auth.oidc_providers[%q].okta.group_role_bindings[%d]: group_name is required", id, j)
			}
			if len(b.Roles) == 0 {
				return fmt.Errorf("auth.oidc_providers[%q].okta.group_role_bindings[%d]: roles must be non-empty", id, j)
			}
			for ri, role := range b.Roles {
				if strings.TrimSpace(role) == "" {
					return fmt.Errorf("auth.oidc_providers[%q].okta.group_role_bindings[%d].roles[%d]: empty role", id, j, ri)
				}
			}
		}
		return nil
	case "entra":
		if p.GitHub != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=entra must not set github block", id)
		}
		if p.GitLab != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=entra must not set gitlab block", id)
		}
		if p.Google != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=entra must not set google block", id)
		}
		if p.Okta != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=entra must not set okta block", id)
		}
		if err := ValidateEntraIssuer(p.Issuer); err != nil {
			return fmt.Errorf("auth.oidc_providers[%q]: kind=entra: %w", id, err)
		}
		e := p.Entra
		if e == nil {
			return nil
		}
		for j, t := range e.AllowedTenantIDs {
			if strings.TrimSpace(t) == "" {
				return fmt.Errorf("auth.oidc_providers[%q].entra.allowed_tenant_ids[%d]: empty entry", id, j)
			}
		}
		for j, g := range e.AllowedIDTokenGroups {
			if strings.TrimSpace(g) == "" {
				return fmt.Errorf("auth.oidc_providers[%q].entra.allowed_id_token_groups[%d]: empty entry", id, j)
			}
		}
		for j, b := range e.GroupRoleBindings {
			if strings.TrimSpace(b.Group) == "" {
				return fmt.Errorf("auth.oidc_providers[%q].entra.group_role_bindings[%d]: group is required", id, j)
			}
			if len(b.Roles) == 0 {
				return fmt.Errorf("auth.oidc_providers[%q].entra.group_role_bindings[%d]: roles must be non-empty", id, j)
			}
			for ri, role := range b.Roles {
				if strings.TrimSpace(role) == "" {
					return fmt.Errorf("auth.oidc_providers[%q].entra.group_role_bindings[%d].roles[%d]: empty role", id, j, ri)
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("auth.oidc_providers[%q]: unknown kind %q (supported: generic, github, gitlab, google, okta, entra)", id, strings.TrimSpace(p.Kind))
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
