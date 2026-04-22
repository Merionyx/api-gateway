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
	if c.Applies(map[string]any{"openapi": 3}) {
		t.Fatal("non-string")
	}
}

const minimalValidOpenAPI3 = `openapi: "3.0.0"
info:
  title: t
  version: "1"
paths: {}
`

func TestRunContentChecks_openapiv3_ok(t *testing.T) {
	t.Parallel()
	root := map[string]any{"openapi": "3.0.0"}
	out := RunContentChecks(context.Background(), "c.yaml", []byte(minimalValidOpenAPI3), root)
	if len(out) != 0 {
		t.Fatalf("%#v", out)
	}
}

func TestRunContentChecks_openapiv3_loadError(t *testing.T) {
	t.Parallel()
	root := map[string]any{"openapi": "3.0.0"}
	out := RunContentChecks(context.Background(), "c.yaml", []byte(`: bad`), root)
	if len(out) != 1 {
		t.Fatalf("want 1 err, got %#v", out)
	}
}
