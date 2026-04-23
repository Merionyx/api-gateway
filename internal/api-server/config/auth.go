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
	}
	return nil
}

// BootstrapAPIKeyAllowed reports whether the insecure bootstrap path may run (roadmap step 10).
func (a AuthConfig) BootstrapAPIKeyAllowed() bool {
	if !a.AllowInsecureBootstrap {
		return false
	}
	e := strings.ToLower(strings.TrimSpace(a.Environment))
	return e == "development" || e == "local"
}
