package resource

import "testing"

func TestResolve_aliases(t *testing.T) {
	for _, tt := range []struct {
		in       string
		wantKind Kind
	}{
		{"controllers", Controllers},
		{"ctrl", Controllers},
		{"cnt", Controllers},
		{"bundles", Bundles},
		{"tenant-bundles", Bundles},
		{"env", Environments},
		{"bundle-keys", BundleKeys},
		{"bk", BundleKeys},
		{"cn", ContractNames},
	} {
		e, ok := Resolve(tt.in)
		if !ok {
			t.Fatalf("Resolve(%q): not found", tt.in)
		}
		if e.Kind != tt.wantKind {
			t.Fatalf("Resolve(%q): got kind %q want %q", tt.in, e.Kind, tt.wantKind)
		}
	}
}
