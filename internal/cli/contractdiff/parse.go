package contractdiff

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseDocument parses YAML or JSON contract bytes into a root map.
func ParseDocument(raw []byte, ext string) (map[string]any, error) {
	ext = strings.ToLower(ext)
	if ext == ".yml" {
		ext = ".yaml"
	}
	var root map[string]any
	switch ext {
	case ".yaml":
		if err := yaml.Unmarshal(raw, &root); err != nil {
			return nil, fmt.Errorf("parse yaml: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(raw, &root); err != nil {
			return nil, fmt.Errorf("parse json: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported extension %q (need .yaml, .yml, or .json)", ext)
	}
	return root, nil
}

// ExtFromPath returns a normalized extension for parsing (.yaml, .json).
func ExtFromPath(p string) string {
	return strings.ToLower(filepath.Ext(p))
}

// ContractNameFromDoc returns x-api-gateway.contract.name.
func ContractNameFromDoc(root map[string]any) (string, error) {
	raw, ok := root["x-api-gateway"]
	if !ok {
		return "", fmt.Errorf("missing top-level %q", "x-api-gateway")
	}
	block, ok := raw.(map[string]any)
	if !ok {
		return "", fmt.Errorf("%q must be a mapping, got %T", "x-api-gateway", raw)
	}
	contract, ok := block["contract"]
	if !ok {
		return "", fmt.Errorf(`x-api-gateway.contract is required`)
	}
	cm, ok := contract.(map[string]any)
	if !ok {
		return "", fmt.Errorf(`x-api-gateway.contract must be a mapping, got %T`, contract)
	}
	name, ok := cm["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return "", fmt.Errorf(`x-api-gateway.contract.name must be a non-empty string`)
	}
	return strings.TrimSpace(name), nil
}
