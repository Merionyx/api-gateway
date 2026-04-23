package config

import "strings"

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
}

// BootstrapAPIKeyAllowed reports whether the insecure bootstrap path may run (roadmap step 10).
func (a AuthConfig) BootstrapAPIKeyAllowed() bool {
	if !a.AllowInsecureBootstrap {
		return false
	}
	e := strings.ToLower(strings.TrimSpace(a.Environment))
	return e == "development" || e == "local"
}
