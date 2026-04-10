package validate

import "testing"

func TestValidateXApiGateway_v1_ok(t *testing.T) {
	root := map[string]any{
		"x-api-gateway": map[string]any{
			"version":                 "v1",
			"prefix":                  "/api/x/",
			"allowUndefinedMethods":   false,
			"contract":                map[string]any{"name": "c"},
			"service":                 map[string]any{"name": "s"},
			"access":                  map[string]any{"secure": true},
		},
	}
	if issues := ValidateXApiGateway(root); len(issues) != 0 {
		t.Fatalf("expected no issues, got %#v", issues)
	}
}

func TestValidateXApiGateway_missingExtension(t *testing.T) {
	root := map[string]any{"openapi": "3.0.0"}
	issues := ValidateXApiGateway(root)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
}

func TestValidateXApiGateway_badVersion(t *testing.T) {
	root := map[string]any{
		"x-api-gateway": map[string]any{
			"version": "v2",
		},
	}
	issues := ValidateXApiGateway(root)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %#v", issues)
	}
}
