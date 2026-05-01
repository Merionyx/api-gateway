// Package credentials stores per-context OIDC tokens for agwctl .
package credentials

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const dirName = "agwctl"

// File holds saved tokens keyed by agwctl context name.
type File struct {
	Contexts map[string]Entry `yaml:"contexts"`
}

// Entry is one context's interactive session material (do not log).
type Entry struct {
	ProviderID               string `yaml:"provider_id"`
	AccessToken              string `yaml:"access_token"`
	RefreshToken             string `yaml:"refresh_token"`
	TokenType                string `yaml:"token_type,omitempty"`
	AccessExpiresAt          string `yaml:"access_expires_at,omitempty"`
	RefreshExpiresAt         string `yaml:"refresh_expires_at,omitempty"`
	RequestedAccessTokenTTL  string `yaml:"requested_access_token_ttl,omitempty"`
	RequestedRefreshTokenTTL string `yaml:"requested_refresh_token_ttl,omitempty"`
	SavedAt                  string `yaml:"saved_at"`
}

// Path returns credentials file path ($AGWCTL_CREDENTIALS or ~/.config/agwctl/credentials.yaml).
func Path() (string, error) {
	if p := strings.TrimSpace(os.Getenv("AGWCTL_CREDENTIALS")); p != "" {
		return p, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".config", dirName, "credentials.yaml"), nil
}

// Load reads credentials; missing file yields empty contexts.
func Load() (*File, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{Contexts: map[string]Entry{}}, nil
		}
		return nil, fmt.Errorf("read credentials %s: %w", p, err)
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse credentials: %w", err)
	}
	if f.Contexts == nil {
		f.Contexts = map[string]Entry{}
	}
	return &f, nil
}

// GetContext returns saved credentials for one agwctl context.
func GetContext(contextName string) (Entry, error) {
	if err := validateContextName(contextName); err != nil {
		return Entry{}, err
	}
	f, err := Load()
	if err != nil {
		return Entry{}, err
	}
	key := strings.TrimSpace(contextName)
	e, ok := f.Contexts[key]
	if !ok {
		return Entry{}, fmt.Errorf("credentials: no tokens saved for context %q; run `agwctl auth login`", key)
	}
	if strings.TrimSpace(e.RefreshToken) == "" {
		return Entry{}, fmt.Errorf("credentials: no refresh token saved for context %q; run `agwctl auth login`", key)
	}
	return e, nil
}

// PutContext merges entry for contextName and writes the file atomically with mode 0600.
func PutContext(contextName string, e Entry) error {
	if err := validateContextName(contextName); err != nil {
		return err
	}
	if strings.TrimSpace(e.AccessToken) == "" || strings.TrimSpace(e.RefreshToken) == "" {
		return fmt.Errorf("credentials: access_token and refresh_token are required")
	}
	if strings.TrimSpace(e.SavedAt) == "" {
		e.SavedAt = time.Now().UTC().Format(time.RFC3339)
	}

	f, err := Load()
	if err != nil {
		return err
	}
	if f.Contexts == nil {
		f.Contexts = map[string]Entry{}
	}
	f.Contexts[strings.TrimSpace(contextName)] = e

	p, err := Path()
	if err != nil {
		return err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename to %s: %w", p, err)
	}
	if err := os.Chmod(p, 0600); err != nil {
		return fmt.Errorf("chmod 0600 %s: %w", p, err)
	}
	return nil
}

func validateContextName(name string) error {
	s := strings.TrimSpace(name)
	if s == "" {
		return fmt.Errorf("context name is empty (set current-context or pass --context)")
	}
	if strings.ContainsAny(s, `/\:`) {
		return fmt.Errorf("context name must not contain / \\ or ':'")
	}
	return nil
}
