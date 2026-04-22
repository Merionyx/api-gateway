package pagination

import (
	"encoding/base64"
	"testing"
)

func TestDecodeOffsetCursor_andResolveLimit(t *testing.T) {
	t.Parallel()
	if o, err := DecodeOffsetCursor(nil); err != nil || o != 0 {
		t.Fatalf("%d %v", o, err)
	}
	empty := ""
	if o, err := DecodeOffsetCursor(&empty); err != nil || o != 0 {
		t.Fatalf("%d %v", o, err)
	}
	bad := "not-base64"
	if _, err := DecodeOffsetCursor(&bad); err == nil {
		t.Fatal("b64")
	}
	nonObject := base64.RawURLEncoding.EncodeToString([]byte(`[1]`))
	if _, err := DecodeOffsetCursor(&nonObject); err == nil {
		t.Fatal("json")
	}
	negB64 := base64.RawURLEncoding.EncodeToString([]byte(`{"o":-1}`))
	if _, err := DecodeOffsetCursor(&negB64); err == nil {
		t.Fatal("negative")
	}
	if l := ResolveLimit(nil); l != DefaultListLimit {
		t.Fatalf("%d", l)
	}
	zero := 0
	if l := ResolveLimit(&zero); l != DefaultListLimit {
		t.Fatalf("%d", l)
	}
	ten := 10
	if l := ResolveLimit(&ten); l != 10 {
		t.Fatalf("%d", l)
	}
}

func TestPageSlice_generics(t *testing.T) {
	t.Parallel()
	all := []int{1, 2, 3, 4, 5}
	lim := 2
	p1, next, more, err := PageSlice(all, lim, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(p1) != 2 || p1[0] != 1 || !more || next == nil {
		t.Fatalf("p1: %#v more=%v", p1, more)
	}
	p2, next2, more2, err := PageSlice(all, lim, next)
	if err != nil {
		t.Fatal(err)
	}
	if len(p2) != 2 || p2[0] != 3 || !more2 || next2 == nil {
		t.Fatalf("p2: %#v", p2)
	}
	p3, next3, more3, err := PageSlice(all, lim, next2)
	if err != nil {
		t.Fatal(err)
	}
	if len(p3) != 1 || p3[0] != 5 || more3 || next3 != nil {
		t.Fatalf("p3: %#v more=%v", p3, more3)
	}
}

func TestPageStringSlice_offsetPastEnd(t *testing.T) {
	t.Parallel()
	items := []string{"a", "b"}
	off := EncodeOffsetCursor(10)
	p, n, m, err := PageStringSlice(items, 5, &off)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 0 || m || n != nil {
		t.Fatalf("p=%v m=%v n=%v", p, m, n)
	}
}
