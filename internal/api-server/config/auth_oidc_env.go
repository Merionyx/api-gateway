package config

import (
	"os"
	"regexp"
	"strings"
)

const oidcCredEnvPrefix = "AGWCP_OIDC_"

var oidcEnvSuffixSanitizer = regexp.MustCompile(`[^a-zA-Z0-9]+`)

// OidcProviderCredentialEnvNames returns env var names for injecting client_id / client_secret from Kubernetes
// (Helm valueFrom secretKeyRef). Names must stay in sync with templates in deployments/helm/api-gateway-control-plane.
func OidcProviderCredentialEnvNames(providerID string) (clientIDEnv, clientSecretEnv string) {
	suf := OidcProviderCredentialEnvSuffix(providerID)
	return oidcCredEnvPrefix + suf + "_CLIENT_ID",
		oidcCredEnvPrefix + suf + "_CLIENT_SECRET"
}

// OidcProviderCredentialEnvSuffix normalizes provider id for env var segment (uppercase, non-alnum → underscore).
func OidcProviderCredentialEnvSuffix(providerID string) string {
	s := strings.TrimSpace(providerID)
	s = oidcEnvSuffixSanitizer.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	return strings.ToUpper(s)
}

// ApplyOIDCProviderSecretsFromEnv overwrites OIDC client_id / client_secret when the corresponding env vars are set.
// Intended for Kubernetes: Secret → env (valueFrom); values in the config file may be empty or placeholders.
// Env wins when non-empty after TrimSpace.
func ApplyOIDCProviderSecretsFromEnv(cfg *Config) {
	if cfg == nil {
		return
	}
	for i := range cfg.Auth.OIDCProviders {
		p := &cfg.Auth.OIDCProviders[i]
		idEnv, secEnv := OidcProviderCredentialEnvNames(p.ID)
		if v := strings.TrimSpace(os.Getenv(idEnv)); v != "" {
			p.ClientID = v
		}
		if v := strings.TrimSpace(os.Getenv(secEnv)); v != "" {
			p.ClientSecret = v
		}
	}
}
