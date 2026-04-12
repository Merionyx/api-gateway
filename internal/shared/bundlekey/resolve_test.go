package bundlekey

import "testing"

func TestResolveFromHTTPQuery_fullKey(t *testing.T) {
	k := "a/b%2Fc/d"
	got, err := ResolveFromHTTPQuery(k, "", "", "")
	if err != nil || got != k {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestResolveFromHTTPQuery_repoRefPath(t *testing.T) {
	got, err := ResolveFromHTTPQuery("", "myrepo", "feature/x", "pkg/api")
	if err != nil {
		t.Fatal(err)
	}
	want := Build("myrepo", "feature/x", "pkg/api")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveFromHTTPQuery_exclusive(t *testing.T) {
	_, err := ResolveFromHTTPQuery("k", "r", "m", "")
	if err == nil {
		t.Fatal("expected error")
	}
}
