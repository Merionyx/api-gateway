package bundlekey

import (
	"strings"
	"testing"
)

func TestEscapeRef(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"main", "main"},
		{"feature/foo", "feature%2Ffoo"},
		{"a/b/c", "a%2Fb%2Fc"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			if got := EscapeRef(tt.ref); got != tt.want {
				t.Errorf("EscapeRef(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}

func TestEscapePath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"", "."},
		{".", "."}, // only empty string maps to "." as segment; "." stays "."
		{"pkg/api", "pkg%2Fapi"},
		{"single", "single"},
	}
	for _, tt := range tests {
		name := tt.path
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			if got := EscapePath(tt.path); got != tt.want {
				t.Errorf("EscapePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestBuildParseRoundTrip(t *testing.T) {
	// Parse splits on every `/`; repository must be a single path segment (no unescaped slash).
	tests := []struct {
		repo, ref, path string
	}{
		{"schemas", "main", ""},
		{"schemas", "main", "bundles/gateway"},
		{"r", "refs/heads/release/v1", "a/b"},
		{"single", "v1.0.0", "root"},
	}
	for _, tt := range tests {
		t.Run(tt.repo+"_"+tt.ref+"_"+tt.path, func(t *testing.T) {
			key := Build(tt.repo, tt.ref, tt.path)
			gotRepo, gotRef, gotPath, err := Parse(key)
			if err != nil {
				t.Fatalf("Parse(Build(...)) err: %v", err)
			}
			if gotRepo != tt.repo || gotRef != tt.ref || gotPath != tt.path {
				t.Errorf("round-trip: got (%q,%q,%q) want (%q,%q,%q)", gotRepo, gotRef, gotPath, tt.repo, tt.ref, tt.path)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		key string
	}{
		{""},
		{"only-two"},
		{"a/b/c/d"},
		{"one"},
	}
	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.key, "/", "_"), func(t *testing.T) {
			_, _, _, err := Parse(tt.key)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "bundle key must be") {
				t.Errorf("error %q should mention bundle key requirement", err.Error())
			}
		})
	}
}

func TestParseDecodesSegments(t *testing.T) {
	// Direct parse of a well-formed key (as Build would produce)
	key := "org/repo%2Fname/pkg%2Fsub"
	repo, ref, path, err := Parse(key)
	if err != nil {
		t.Fatal(err)
	}
	if repo != "org" || ref != "repo/name" || path != "pkg/sub" {
		t.Errorf("got (%q,%q,%q)", repo, ref, path)
	}
}
