package bundlekey

import (
	"fmt"
	"strings"
)

// ResolveFromHTTPQuery builds the canonical bundle key from either a full `bundle_key` string
// or from repo + ref + optional logical path (same as Build). All inputs are trimmed.
// It returns an error if both styles are mixed, or if neither style is complete.
func ResolveFromHTTPQuery(bundleKey, repo, ref, path string) (string, error) {
	bk := strings.TrimSpace(bundleKey)
	r := strings.TrimSpace(repo)
	f := strings.TrimSpace(ref)
	p := strings.TrimSpace(path)

	hasBK := bk != ""
	hasRepoRef := r != "" && f != ""
	partialRepo := (r != "" || f != "" || p != "") && !hasRepoRef

	if hasBK && (r != "" || f != "" || p != "") {
		return "", fmt.Errorf("use either bundle_key or repo/ref/path, not both")
	}
	if partialRepo {
		return "", fmt.Errorf("repo and ref must both be set when using repo/ref/path")
	}
	if hasBK {
		return NormalizeQueryDecodedBundleKey(bk)
	}
	if hasRepoRef {
		return Build(r, f, p), nil
	}
	return "", fmt.Errorf("set bundle_key or repo and ref (path optional)")
}
