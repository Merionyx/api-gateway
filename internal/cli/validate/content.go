package validate

import (
	"context"
	"fmt"
	"strings"
)

// ContentChecker validates document body beyond x-api-gateway (e.g. OpenAPI schema).
type ContentChecker interface {
	// ID is a short label for messages, e.g. "openapi".
	ID() string
	// Applies returns whether this checker should run on the parsed root map.
	Applies(root map[string]any) bool
	// Check validates raw bytes (same as on disk) for documents that Applies.
	Check(ctx context.Context, path string, data []byte) error
}

var contentCheckers = []ContentChecker{
	openAPIChecker{},
}

// RunContentChecks runs all registered content checkers that apply. Errors are prefixed with [content:<id>].
func RunContentChecks(ctx context.Context, path string, data []byte, root map[string]any) []string {
	var out []string
	for _, c := range contentCheckers {
		if !c.Applies(root) {
			continue
		}
		if err := c.Check(ctx, path, data); err != nil {
			out = append(out, fmt.Sprintf("[content:%s] %v", c.ID(), err))
		}
	}
	return out
}

type openAPIChecker struct{}

func (openAPIChecker) ID() string { return "openapi" }

func (openAPIChecker) Applies(root map[string]any) bool {
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
