// Package bundleopt resolves bundle identity from agwctl flags (--bundle-key vs --repo/--ref/--path).
package bundleopt

import (
	"fmt"
	"strings"

	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
)

// ResolveBundleKey returns the canonical bundle key from flags only (for contract-names).
func ResolveBundleKey(bundleKeyFlag, repo, ref, path string) (string, error) {
	return bundlekey.ResolveFromHTTPQuery(
		strings.TrimSpace(bundleKeyFlag),
		strings.TrimSpace(repo),
		strings.TrimSpace(ref),
		strings.TrimSpace(path),
	)
}

// ResolveBundleKeyOrName resolves bundle key from flags, or from positional NAME when flags are absent
// (for describe bundle-keys).
func ResolveBundleKeyOrName(bundleKeyFlag, repo, ref, path, nameArg string) (string, error) {
	bk := strings.TrimSpace(bundleKeyFlag)
	r := strings.TrimSpace(repo)
	f := strings.TrimSpace(ref)
	p := strings.TrimSpace(path)
	n := strings.TrimSpace(nameArg)

	if bk != "" && (r != "" || f != "" || p != "") {
		return "", fmt.Errorf("use either --bundle-key or --repo/--ref/--path, not both")
	}
	hasRR := r != "" && f != ""
	if (r != "" || f != "" || p != "") && !hasRR {
		return "", fmt.Errorf("repo and ref must both be set when using --repo/--ref/--path")
	}
	if bk != "" || hasRR {
		return bundlekey.ResolveFromHTTPQuery(bk, r, f, p)
	}
	if n != "" {
		return n, nil
	}
	return "", fmt.Errorf("set --bundle-key or (--repo and --ref and optional --path), or pass the bundle key as NAME")
}
