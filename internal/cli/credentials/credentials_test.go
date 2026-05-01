package credentials

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPutContext_roundTripAndMode0600(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGWCTL_CREDENTIALS", filepath.Join(dir, "creds.yaml"))

	e := Entry{
		ProviderID:               "p1",
		AccessToken:              "at-test",
		RefreshToken:             "rt-test",
		TokenType:                "Bearer",
		AccessExpiresAt:          "2026-04-30T12:00:00Z",
		RefreshExpiresAt:         "2026-05-23T12:00:00Z",
		RequestedAccessTokenTTL:  "168h",
		RequestedRefreshTokenTTL: "720h",
		SavedAt:                  "2026-04-23T12:00:00Z",
	}
	if err := PutContext("dev", e); err != nil {
		t.Fatal(err)
	}
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if m := st.Mode().Perm(); m != 0600 {
		t.Fatalf("file mode want 0600 got %o", m)
	}
	f, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	got, ok := f.Contexts["dev"]
	if !ok {
		t.Fatalf("contexts %+v", f.Contexts)
	}
	if got.AccessToken != e.AccessToken || got.RefreshToken != e.RefreshToken || got.RequestedAccessTokenTTL != e.RequestedAccessTokenTTL || got.RefreshExpiresAt != e.RefreshExpiresAt {
		t.Fatalf("got %+v", got)
	}
}

func TestPutContext_emptyContext(t *testing.T) {
	t.Parallel()
	err := PutContext("  ", Entry{AccessToken: "a", RefreshToken: "b"})
	if err == nil {
		t.Fatal("expected error")
	}
}
