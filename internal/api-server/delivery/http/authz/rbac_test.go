package authz

import "testing"

func TestNormalizeRolesValue(t *testing.T) {
	t.Parallel()
	if got := NormalizeRolesValue(nil); len(got) != 0 {
		t.Fatalf("nil: %#v", got)
	}
	if got := NormalizeRolesValue([]string{" a ", "b"}); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("[]string: %#v", got)
	}
	if got := NormalizeRolesValue([]any{"x", 1, "y"}); len(got) != 2 || got[0] != "x" || got[1] != "y" {
		t.Fatalf("[]any: %#v", got)
	}
}
