package config

import (
	"fmt"
	"strings"
)

// DevCORSAllowOrigins is the default browser Origin list for local development (no "*").
func DevCORSAllowOrigins() []string {
	return []string{
		"http://127.0.0.1:3000",
		"http://localhost:3000",
		"http://127.0.0.1:5173",
		"http://localhost:5173",
		"http://127.0.0.1:8080",
		"http://localhost:8080",
	}
}

// ApplyCORSDevDefaults fills allow_origins when unset/empty and auth.environment is development|local.
func ApplyCORSDevDefaults(cfg *Config) {
	if len(NormalizeCORSAllowOrigins(cfg.Server.CORS.AllowOrigins)) != 0 {
		return
	}
	e := strings.ToLower(strings.TrimSpace(cfg.Auth.Environment))
	if e == "development" || e == "local" {
		cfg.Server.CORS.AllowOrigins = DevCORSAllowOrigins()
	}
}

// CORSConfig controls browser cross-origin access (roadmap ш. 24).
// AllowOrigins is an exact-match list for the `Origin` request header (scheme + host + optional port).
// Use InsecureAllowWildcard only in local/dev; never in production or staging.
type CORSConfig struct {
	AllowOrigins []string `mapstructure:"allow_origins" json:"allow_origins"`
	// InsecureAllowWildcard permits literal "*" in AllowOrigins when combined with a non-production auth.environment.
	InsecureAllowWildcard bool `mapstructure:"insecure_allow_wildcard" json:"insecure_allow_wildcard"`
}

// NormalizeCORSAllowOrigins trims entries, drops empties, and de-duplicates while preserving order.
func NormalizeCORSAllowOrigins(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		o := strings.TrimSpace(raw)
		if o == "" {
			continue
		}
		if _, ok := seen[o]; ok {
			continue
		}
		seen[o] = struct{}{}
		out = append(out, o)
	}
	return out
}

// ValidateServerCORS rejects unsafe wildcard combinations (align with AuthConfig.Environment rules).
func ValidateServerCORS(c CORSConfig, authEnvironment string) error {
	origins := NormalizeCORSAllowOrigins(c.AllowOrigins)
	var hasStar, hasOther bool
	for _, o := range origins {
		if o == "*" {
			hasStar = true
			continue
		}
		hasOther = true
	}
	if hasStar && hasOther {
		return fmt.Errorf(`server.cors: cannot mix "*" with other origins in allow_origins`)
	}
	if !hasStar {
		return nil
	}
	if !c.InsecureAllowWildcard {
		return fmt.Errorf(`server.cors: allow_origins contains "*" — set server.cors.insecure_allow_wildcard=true only for local/dev, or list explicit origins`)
	}
	e := strings.ToLower(strings.TrimSpace(authEnvironment))
	if e == "production" || e == "staging" {
		return fmt.Errorf(`server.cors: wildcard Origin is not allowed when auth.environment is %q`, authEnvironment)
	}
	return nil
}
