package usecase

import (
	"testing"
)

func TestPageStringSlice(t *testing.T) {
	items := []string{"a", "b", "c", "d"}
	lim := 2
	first, next, more, err := PageStringSlice(items, lim, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || first[0] != "a" || first[1] != "b" {
		t.Fatalf("first page: %#v", first)
	}
	if !more || next == nil {
		t.Fatalf("expected more")
	}
	second, _, more2, err := PageStringSlice(items, lim, next)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 2 || second[0] != "c" || second[1] != "d" {
		t.Fatalf("second page: %#v", second)
	}
	if more2 {
		t.Fatalf("unexpected more on last page")
	}
}
