package contractfmt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"gopkg.in/yaml.v3"
)

// FormatBytes canonicalizes contract bytes. ext is a file extension such as ".yaml" or ".json".
// outFormat is "yaml" (default), "yml", or "json".
func FormatBytes(data []byte, ext, outFormat string) ([]byte, error) {
	ext = normalizeExt(ext)
	outFormat = strings.ToLower(strings.TrimSpace(outFormat))
	if outFormat == "" {
		outFormat = "yaml"
	}
	if outFormat == "yml" {
		outFormat = "yaml"
	}

	root, err := parseRoot(data, ext)
	if err != nil {
		return nil, err
	}
	kind, err := DetectKind(root)
	if err != nil {
		return nil, err
	}

	var yml []byte
	switch kind {
	case KindXAGOnly:
		yml, err = formatXAGOnly(root)
	case KindOpenAPI:
		yml, err = formatOpenAPI(root)
	default:
		return nil, fmt.Errorf("unsupported contract kind")
	}
	if err != nil {
		return nil, err
	}

	switch outFormat {
	case "yaml":
		return yml, nil
	case "json":
		return yamlToJSON(yml)
	default:
		return nil, fmt.Errorf("invalid output format %q (use yaml or json)", outFormat)
	}
}

func parseRoot(data []byte, ext string) (map[string]any, error) {
	var root map[string]any
	switch ext {
	case ".yaml":
		if err := yaml.Unmarshal(data, &root); err != nil {
			return nil, fmt.Errorf("parse yaml: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &root); err != nil {
			return nil, fmt.Errorf("parse json: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported extension %q", ext)
	}
	if root == nil {
		return nil, fmt.Errorf("empty document")
	}
	return root, nil
}

func formatXAGOnly(root map[string]any) ([]byte, error) {
	raw, ok := root[extKey]
	if !ok {
		return nil, fmt.Errorf("missing top-level %q", extKey)
	}
	block, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%q must be a mapping, got %T", extKey, raw)
	}
	return encodeYAMLOrdered([]orderedPair{{key: extKey, val: XAGDoc(block)}})
}

func formatOpenAPI(root map[string]any) ([]byte, error) {
	doc, err := loadOpenAPIDocument(root)
	if err != nil {
		return nil, err
	}

	var xag XAGDoc
	hasXAG := false
	if doc.Extensions != nil {
		if raw, ok := doc.Extensions[extKey]; ok && raw != nil {
			block, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s must be a mapping, got %T", extKey, raw)
			}
			xag = XAGDoc(block)
			hasXAG = true
			delete(doc.Extensions, extKey)
		}
	}

	pairs, err := appendOpenAPIPairs(doc, xag, hasXAG)
	if err != nil {
		return nil, err
	}
	return encodeYAMLOrdered(pairs)
}

// loadOpenAPIDocument builds *openapi3.T from the root map produced by parseRoot (single YAML/JSON decode per FormatBytes).
// It mirrors kin-openapi Loader.LoadFromData without reparsing raw bytes; edge-case type differences vs a direct byte decode are unlikely for typical specs.
func loadOpenAPIDocument(root map[string]any) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	b, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("load OpenAPI: %w", err)
	}
	var doc openapi3.T
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("load OpenAPI: %w", err)
	}
	if err := loader.ResolveRefsIn(&doc, nil); err != nil {
		return nil, fmt.Errorf("load OpenAPI: %w", err)
	}
	return &doc, nil
}

func yamlToJSON(yml []byte) ([]byte, error) {
	var v any
	if err := yaml.Unmarshal(yml, &v); err != nil {
		return nil, err
	}
	// WARN: YAML tags, anchors, and non-JSON types are lost in this round-trip.
	return json.MarshalIndent(v, "", "  ")
}
