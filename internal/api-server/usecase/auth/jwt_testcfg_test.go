package auth

import (
	"path/filepath"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/config"
)

// jwtTestCfg builds a JWTConfig with distinct API and Edge key directories under apiKeysRoot.
func jwtTestCfg(t *testing.T, apiKeysRoot string) *config.JWTConfig {
	t.Helper()
	return &config.JWTConfig{
		KeysDir:      apiKeysRoot,
		EdgeKeysDir:  filepath.Join(apiKeysRoot, "edge"),
		Issuer:       "iss",
		APIAudience:  "api-aud",
		EdgeIssuer:   "edge-iss",
		EdgeAudience: "edge-aud",
	}
}
