package config

import (
	"os"
	"strings"
)

// ApplySessionKekFromEnv sets auth.session_kek_base64 from API_SERVER_AUTH_SESSION_KEK_BASE64.
//
// Helm injects that env via secretKeyRef while the merged YAML often omits session_kek_base64
// entirely. Viper's Unmarshal only pulls env for keys present in its flattened key set, so
// nested secrets would otherwise never apply. Non-empty env wins over any file value.
func ApplySessionKekFromEnv(cfg *Config) {
	if s := strings.TrimSpace(os.Getenv("API_SERVER_AUTH_SESSION_KEK_BASE64")); s != "" {
		cfg.Auth.SessionKEKBase64 = s
	}
}
