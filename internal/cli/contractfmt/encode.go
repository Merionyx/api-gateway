package contractfmt

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"gopkg.in/yaml.v3"
)

func normalizeExt(ext string) string {
	e := strings.ToLower(strings.TrimSpace(ext))
	if e == ".yml" {
		return ".yaml"
	}
	return e
}

// encodeYAMLOrdered builds a YAML document from ordered key/value pairs (stable key order).
func encodeYAMLOrdered(pairs []orderedPair) ([]byte, error) {
	var doc yaml.Node
	doc.Kind = yaml.DocumentNode
	doc.HeadComment = ""

	var mapping yaml.Node
	mapping.Kind = yaml.MappingNode
	mapping.Style = 0

	for _, p := range pairs {
		var keyNode yaml.Node
		if err := keyNode.Encode(p.key); err != nil {
			return nil, fmt.Errorf("yaml key %q: %w", p.key, err)
		}

		valNode, err := encodeYAMLNode(p.val)
		if err != nil {
			return nil, fmt.Errorf("yaml value %q: %w", p.key, err)
		}
		mapping.Content = append(mapping.Content, &keyNode, valNode)
	}
	doc.Content = append(doc.Content, &mapping)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type orderedPair struct {
	key string
	val any
}

func sortedStringKeys(m map[string]any) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func appendOpenAPIPairs(doc *openapi3.T, xag XAGDoc, hasXAG bool) ([]orderedPair, error) {
	if doc == nil {
		return nil, fmt.Errorf("nil OpenAPI document")
	}
	extCopy := map[string]any{}
	if doc.Extensions != nil {
		for k, v := range doc.Extensions {
			// formatOpenAPI already removed extKey; keep the check for callers that pass a raw doc.
			if k == extKey {
				continue
			}
			extCopy[k] = v
		}
	}

	var pairs []orderedPair
	pairs = append(pairs, orderedPair{key: "openapi", val: doc.OpenAPI})

	if doc.Info != nil {
		pairs = append(pairs, orderedPair{key: "info", val: doc.Info})
	}
	if doc.Paths != nil {
		pairs = append(pairs, orderedPair{key: "paths", val: doc.Paths})
	}
	if doc.Components != nil {
		pairs = append(pairs, orderedPair{key: "components", val: doc.Components})
	}
	if len(doc.Security) > 0 {
		pairs = append(pairs, orderedPair{key: "security", val: doc.Security})
	}
	if len(doc.Servers) > 0 {
		pairs = append(pairs, orderedPair{key: "servers", val: doc.Servers})
	}
	if len(doc.Tags) > 0 {
		pairs = append(pairs, orderedPair{key: "tags", val: doc.Tags})
	}
	if doc.ExternalDocs != nil {
		pairs = append(pairs, orderedPair{key: "externalDocs", val: doc.ExternalDocs})
	}

	for _, k := range sortedStringKeys(extCopy) {
		pairs = append(pairs, orderedPair{key: k, val: extCopy[k]})
	}
	if hasXAG {
		if len(xag) == 0 {
			return nil, fmt.Errorf("internal: hasXAG but x-api-gateway block is empty")
		}
		pairs = append(pairs, orderedPair{key: extKey, val: xag})
	}
	return pairs, nil
}
