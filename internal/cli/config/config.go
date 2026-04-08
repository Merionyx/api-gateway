// Package config loads kubeconfig-style agwctl settings (context → API Server URL).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const dirName = "agwctl"

// File is kubeconfig-like: contexts with server URL only.
type File struct {
	CurrentContext string                   `yaml:"current-context"`
	Contexts       map[string]ContextConfig `yaml:"contexts"`
}

// ContextConfig holds one named context.
type ContextConfig struct {
	Server string `yaml:"server"`
}

// Path returns the config file path ($AGWCTL_CONFIG or ~/.config/agwctl/config.yaml).
func Path() (string, error) {
	if p := strings.TrimSpace(os.Getenv("AGWCTL_CONFIG")); p != "" {
		return p, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".config", dirName, "config.yaml"), nil
}

// Load reads and parses the config file; missing file yields empty contexts.
func Load() (*File, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{Contexts: map[string]ContextConfig{}}, nil
		}
		return nil, fmt.Errorf("read config %s: %w", p, err)
	}
	var c File
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if c.Contexts == nil {
		c.Contexts = map[string]ContextConfig{}
	}
	return &c, nil
}

// ResolveServerURL returns --server override if set, otherwise the server URL for the given or current context.
func ResolveServerURL(contextName, serverOverride string) (string, error) {
	if strings.TrimSpace(serverOverride) != "" {
		return strings.TrimSpace(serverOverride), nil
	}
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(contextName)
	if name == "" {
		name = strings.TrimSpace(cfg.CurrentContext)
	}
	if name == "" {
		return "", fmt.Errorf("no server: set --server or context in %s (current-context)", pathHint())
	}
	ctx, ok := cfg.Contexts[name]
	if !ok || strings.TrimSpace(ctx.Server) == "" {
		return "", fmt.Errorf("context %q has no server in config", name)
	}
	return strings.TrimSpace(ctx.Server), nil
}

func pathHint() string {
	p, err := Path()
	if err != nil {
		return "~/.config/agwctl/config.yaml"
	}
	return p
}
