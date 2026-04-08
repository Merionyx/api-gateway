// Package convertfmt converts OpenAPI YAML/JSON bytes for agwctl --format.
package convertfmt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// NormalizeExtFromSourcePath returns .yaml, .yml, or .json from a file path.
func NormalizeExtFromSourcePath(sourcePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(sourcePath))
	switch ext {
	case ".yaml", ".yml", ".json":
		return ext, nil
	default:
		return "", fmt.Errorf("unsupported source extension %q (need .yaml, .yml, or .json)", ext)
	}
}

// OutputExt returns the file extension for output: from --format or from source path.
func OutputExt(sourcePath, formatFlag string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(formatFlag)) {
	case "yaml", "yml":
		return ".yaml", nil
	case "json":
		return ".json", nil
	case "":
		return NormalizeExtFromSourcePath(sourcePath)
	default:
		return "", fmt.Errorf("invalid --format %q (use yaml or json)", formatFlag)
	}
}

// Convert parses content according to sourceExt and serializes to target format ("yaml"|"json"|"yml").
func Convert(content []byte, sourceExt, targetFormat string) ([]byte, error) {
	target := strings.ToLower(strings.TrimSpace(targetFormat))
	if target != "yaml" && target != "yml" && target != "json" {
		return nil, fmt.Errorf("target format %q", targetFormat)
	}
	var doc any
	switch strings.ToLower(sourceExt) {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(content, &doc); err != nil {
			return nil, fmt.Errorf("parse yaml: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(content, &doc); err != nil {
			return nil, fmt.Errorf("parse json: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown source ext %q", sourceExt)
	}
	if target == "json" {
		return json.MarshalIndent(doc, "", "  ")
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
