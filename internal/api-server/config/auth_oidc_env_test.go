package config

import (
	"testing"
)

func TestOidcProviderCredentialEnvSuffix(t *testing.T) {
	t.Parallel()
	if g := OidcProviderCredentialEnvSuffix("github"); g != "GITHUB" {
		t.Fatalf("%q", g)
	}
	if g := OidcProviderCredentialEnvSuffix("my-github"); g != "MY_GITHUB" {
		t.Fatalf("%q", g)
	}
}

func TestApplyOIDCProviderSecretsFromEnv(t *testing.T) {
	// No t.Parallel: t.Setenv is incompatible with parallel tests (Go 1.26+).
	idVar, secVar := OidcProviderCredentialEnvNames("github")
	t.Setenv(idVar, "id-from-env")
	t.Setenv(secVar, "secret-from-env")
	cfg := &Config{
		Auth: AuthConfig{
			OIDCProviders: []OIDCProviderConfig{{
				ID:           "github",
				Issuer:       "https://github.com/login/oauth",
				ClientID:     "was-file",
				ClientSecret: "was-secret",
				RedirectURIAllowlist: []string{"http://127.0.0.1/cb"},
				Kind:         "github",
			}},
		},
	}
	ApplyOIDCProviderSecretsFromEnv(cfg)
	p := cfg.Auth.OIDCProviders[0]
	if p.ClientID != "id-from-env" || p.ClientSecret != "secret-from-env" {
		t.Fatalf("%q %q", p.ClientID, p.ClientSecret)
	}
}
