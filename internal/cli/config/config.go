// Package config loads kubeconfig-style agwctl settings (context → API Server URL).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// Save writes the config file atomically (0644 on file, 0755 on parent dirs). Creates ~/.config/agwctl if needed.
func Save(f *File) error {
	if f == nil {
		return fmt.Errorf("config: nil file")
	}
	if f.Contexts == nil {
		f.Contexts = map[string]ContextConfig{}
	}
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
		return fmt.Errorf("encode config: %w", err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename to %s: %w", p, err)
	}
	return nil
}

// ValidateContextName returns an error if name is empty or not a safe context id.
func ValidateContextName(name string) error {
	s := strings.TrimSpace(name)
	if s == "" {
		return fmt.Errorf("context name is empty")
	}
	if strings.ContainsAny(s, `/\:`) {
		return fmt.Errorf("context name must not contain / \\ or :")
	}
	return nil
}

// SetContext adds or updates a named context with server URL (trimmed). Does not change current-context.
func (f *File) SetContext(name, server string) error {
	if err := ValidateContextName(name); err != nil {
		return err
	}
	s := strings.TrimSpace(server)
	if s == "" {
		return fmt.Errorf("server URL is empty")
	}
	if f.Contexts == nil {
		f.Contexts = map[string]ContextConfig{}
	}
	f.Contexts[strings.TrimSpace(name)] = ContextConfig{Server: s}
	return nil
}

// DeleteContext removes a context. If it was current, clears current-context. Returns true if a context was removed.
func (f *File) DeleteContext(name string) (bool, error) {
	if err := ValidateContextName(name); err != nil {
		return false, err
	}
	key := strings.TrimSpace(name)
	if f.Contexts == nil {
		return false, nil
	}
	if _, ok := f.Contexts[key]; !ok {
		return false, nil
	}
	delete(f.Contexts, key)
	if strings.TrimSpace(f.CurrentContext) == key {
		f.CurrentContext = ""
	}
	return true, nil
}

// UseContext sets current-context to name; the context must already exist with a non-empty server.
func (f *File) UseContext(name string) error {
	if err := ValidateContextName(name); err != nil {
		return err
	}
	key := strings.TrimSpace(name)
	if f.Contexts == nil {
		return fmt.Errorf("no context %q", key)
	}
	ctx, ok := f.Contexts[key]
	if !ok {
		return fmt.Errorf("no context %q", key)
	}
	if strings.TrimSpace(ctx.Server) == "" {
		return fmt.Errorf("context %q has no server; run set-context with --server", key)
	}
	f.CurrentContext = key
	return nil
}

// ContextNames returns sorted context names.
func (f *File) ContextNames() []string {
	if f == nil || len(f.Contexts) == 0 {
		return nil
	}
	out := make([]string, 0, len(f.Contexts))
	for n := range f.Contexts {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
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
