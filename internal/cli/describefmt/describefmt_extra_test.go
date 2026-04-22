package describefmt

import (
	"strings"
	"testing"
)

func TestWrite_emptyMapAndSlice(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	if err := Write(&b, map[string]any{}, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "{}") {
		t.Fatalf("%q", b.String())
	}
	b.Reset()
	if err := Write(&b, map[string]any{"arr": []any{}}, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "[]") {
		t.Fatalf("%q", b.String())
	}
}

func TestWrite_nestedListAndScalarTypes(t *testing.T) {
	t.Parallel()
	v := map[string]any{
		"n": nil,
		"f": 1.5,
		"nested": []any{
			[]any{1, 2},
		},
		"item": map[string]any{
			"x": map[string]any{},
		},
	}
	var b strings.Builder
	if err := Write(&b, v, false); err != nil {
		t.Fatal(err)
	}
	s := b.String()
	if !strings.Contains(s, "null") || !strings.Contains(s, "1.5") {
		t.Fatalf("%q", s)
	}
}

func TestNormalize_sliceBranch(t *testing.T) {
	t.Parallel()
	n := Normalize([]any{map[any]any{1: 2}, "s"})
	_, ok := n.([]any)
	if !ok {
		t.Fatalf("%T", n)
	}
}

func TestScalarString_types(t *testing.T) {
	t.Parallel()
	if scalarString(int32(0)) == "" {
		t.Fatal("int32 via default")
	}
}
