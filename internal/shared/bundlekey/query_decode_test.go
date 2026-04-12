package bundlekey

import "testing"

func TestNormalizeQueryDecodedBundleKey_canonicalUnchanged(t *testing.T) {
	want := Build("api-gateway-schemas-https", "remotes/origin/master", "openapi")
	got, err := NormalizeQueryDecodedBundleKey(want)
	if err != nil || got != want {
		t.Fatalf("got %q err %v want %q", got, err, want)
	}
}

func TestNormalizeQueryDecodedBundleKey_curlStyleOverDecoded(t *testing.T) {
	// As if query had bundle_key=api-gateway-schemas-https%2Fremotes%2Forigin%2Fmaster%2Fopenapi
	// and %2F was decoded to / everywhere.
	s := "api-gateway-schemas-https/remotes/origin/master/openapi"
	want := Build("api-gateway-schemas-https", "remotes/origin/master", "openapi")
	got, err := NormalizeQueryDecodedBundleKey(s)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNormalizeQueryDecodedBundleKey_rootPath(t *testing.T) {
	s := "api-gateway-schemas-https/remotes/origin/master/."
	want := Build("api-gateway-schemas-https", "remotes/origin/master", "")
	got, err := NormalizeQueryDecodedBundleKey(s)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
