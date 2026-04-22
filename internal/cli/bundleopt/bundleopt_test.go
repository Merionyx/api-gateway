package bundleopt

import "testing"

func TestResolveBundleKey(t *testing.T) {
	t.Parallel()
	k, err := ResolveBundleKey("", "r", "main", "p")
	if err != nil {
		t.Fatal(err)
	}
	if k == "" {
		t.Fatal("key")
	}
}

func TestResolveBundleKeyOrName(t *testing.T) {
	t.Parallel()
	if _, err := ResolveBundleKeyOrName("bk", "r", "", "", ""); err == nil {
		t.Fatal("conflict bundle-key and repo")
	}
	if _, err := ResolveBundleKeyOrName("", "r", "", "p", ""); err == nil {
		t.Fatal("repo without ref")
	}
	k, err := ResolveBundleKeyOrName("", "r", "f", "p", "")
	if err != nil {
		t.Fatal(err)
	}
	if k == "" {
		t.Fatal()
	}
	n, err := ResolveBundleKeyOrName("", "", "", "", "  my-key  ")
	if err != nil || n != "my-key" {
		t.Fatalf("%q %v", n, err)
	}
	if _, err := ResolveBundleKeyOrName("", "", "", "", ""); err == nil {
		t.Fatal("nothing set")
	}
}
