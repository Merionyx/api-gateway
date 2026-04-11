package handler

import (
	"testing"
)

func TestJSONETag(t *testing.T) {
	t.Parallel()
	etag, err := jsonETag(map[string]int{"a": 1})
	if err != nil {
		t.Fatal(err)
	}
	if etag == "" || etag[0] != '"' {
		t.Fatalf("etag %q", etag)
	}
}

func TestIfNoneMatchMatches(t *testing.T) {
	t.Parallel()
	etag := `"deadbeef"`
	if !ifNoneMatchMatches(`deadbeef`, etag) {
		t.Fatal("plain hex")
	}
	if !ifNoneMatchMatches(`W/"deadbeef"`, etag) {
		t.Fatal("weak etag")
	}
	if ifNoneMatchMatches(`"cafe"`, etag) {
		t.Fatal("mismatch")
	}
}
