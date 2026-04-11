package version

import (
	"strings"
	"testing"
)

func TestLine(t *testing.T) {
	savedV, savedC := Version, Commit
	defer func() {
		Version, Commit = savedV, savedC
	}()

	Version, Commit = "  ", ""
	if got := Line(); got != "dev" {
		t.Fatalf("empty trim: %q", got)
	}
	Version, Commit = "1.0.0", "abc"
	if got := Line(); got != "1.0.0 (abc)" {
		t.Fatalf("got %q", got)
	}
}

func TestDetails(t *testing.T) {
	savedV, savedC, savedT := Version, Commit, BuildTime
	defer func() {
		Version, Commit, BuildTime = savedV, savedC, savedT
	}()

	Version, Commit, BuildTime = "2.0.0", "def", "2024-01-01"
	out := Details()
	if !strings.Contains(out, "agwctl 2.0.0") || !strings.Contains(out, "commit: def") {
		t.Fatalf("missing version/commit:\n%s", out)
	}
	if !strings.Contains(out, "build: 2024-01-01") || !strings.Contains(out, "go: ") {
		t.Fatalf("missing build/go:\n%s", out)
	}
}
