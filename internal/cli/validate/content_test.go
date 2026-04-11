package validate

import (
	"context"
	"testing"
)

func TestRunContentChecks_skipsNonOpenAPI(t *testing.T) {
	t.Parallel()
	root := map[string]any{"other": true}
	if out := RunContentChecks(context.Background(), "p.yaml", []byte(`{}`), root); len(out) != 0 {
		t.Fatalf("got %#v", out)
	}
}

func TestOpenAPIChecker_Applies(t *testing.T) {
	t.Parallel()
	var c openAPIChecker
	if c.Applies(map[string]any{"openapi": "2.0"}) {
		t.Fatal("v2 should not apply")
	}
	if !c.Applies(map[string]any{"openapi": "3.0.0"}) {
		t.Fatal("v3 should apply")
	}
}
