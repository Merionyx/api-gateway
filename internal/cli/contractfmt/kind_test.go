package contractfmt

import "testing"

func TestDetectKind(t *testing.T) {
	t.Parallel()
	_, err := DetectKind(nil)
	if err == nil {
		t.Fatal("nil root")
	}
	k, err := DetectKind(map[string]any{
		"openapi": "3.0.0",
		"other":   true,
	})
	if err != nil || k != KindOpenAPI {
		t.Fatalf("k=%v err=%v", k, err)
	}
	k, err = DetectKind(map[string]any{
		"x-api-gateway": map[string]any{"version": "v1"},
	})
	if err != nil || k != KindXAGOnly {
		t.Fatalf("k=%v err=%v", k, err)
	}
	_, err = DetectKind(map[string]any{"nope": 1})
	if err == nil {
		t.Fatal("unknown")
	}
	_, err = DetectKind(map[string]any{"openapi": "2.0"})
	if err == nil {
		t.Fatal("oas2 should not be KindOpenAPI")
	}
}
