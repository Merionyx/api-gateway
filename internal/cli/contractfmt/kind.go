package contractfmt

import (
	"fmt"
	"strings"
)

const extKey = "x-api-gateway"

// Kind describes how a contract file should be formatted.
type Kind int

const (
	KindUnknown Kind = iota
	KindXAGOnly
	KindOpenAPI
)

// DetectKind classifies the document root map.
// OpenAPI 3.x wins when present; otherwise x-api-gateway alone is KindXAGOnly.
// WARN: OpenAPI 2.x / Swagger (field swagger:) is not supported — returns KindUnknown with an error.
func DetectKind(root map[string]any) (Kind, error) {
	if root == nil {
		return KindUnknown, fmt.Errorf("empty document")
	}
	if isOpenAPI3(root) {
		return KindOpenAPI, nil
	}
	if _, ok := root[extKey]; ok {
		return KindXAGOnly, nil
	}
	return KindUnknown, fmt.Errorf("need top-level %q or OpenAPI 3.x (field %q)", extKey, "openapi")
}

func isOpenAPI3(root map[string]any) bool {
	v, ok := root["openapi"]
	if !ok || v == nil {
		return false
	}
	s, ok := v.(string)
	if !ok {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(s), "3.")
}
