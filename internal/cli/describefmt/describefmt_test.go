package describefmt

import (
	"strings"
	"testing"
)

func TestWrite_scalarsOneSpace(t *testing.T) {
	v := map[string]any{
		"name":      "pod-1",
		"namespace": "default",
		"ready":     true,
	}
	var b strings.Builder
	_ = Write(&b, v, false)
	got := b.String()
	if !strings.Contains(got, "name: pod-1") {
		t.Fatalf("expected name line, got:\n%s", got)
	}
	if strings.Contains(got, "name:      ") {
		t.Fatalf("unexpected column padding, got:\n%s", got)
	}
}

func TestWrite_listOfMapsYAMLStyle(t *testing.T) {
	v := map[string]any{
		"apps": []any{
			map[string]any{"app_id": "app-1", "env": "dev"},
		},
	}
	var b strings.Builder
	_ = Write(&b, v, false)
	got := b.String()
	if !strings.Contains(got, "- app_id: app-1") {
		t.Fatalf("want merged list map line, got:\n%s", got)
	}
}

func TestNormalize_mapAnyKey(t *testing.T) {
	raw := map[any]any{"a": 1, "b": map[any]any{"c": 2}}
	n := Normalize(raw)
	m, ok := n.(map[string]any)
	if !ok {
		t.Fatalf("got %T", n)
	}
	if m["a"] != 1 {
		t.Fatalf("a=%v", m["a"])
	}
}

func TestWrite_colorDoesNotPanic(t *testing.T) {
	v := map[string]any{"k": "v"}
	var b strings.Builder
	_ = Write(&b, v, true)
	if !strings.Contains(b.String(), "v") {
		t.Fatalf("output: %q", b.String())
	}
}
