package bundlekey

import (
	"fmt"
	"strings"
)

// NormalizeQueryDecodedBundleKey maps a bundle_key value after standard URL query decoding back to
// the canonical etcd key. Query parsers decode "%2F" to "/", which breaks the three-segment
// convention (ref and path segments use literal "%2F" for slashes). When Parse succeeds, s is
// already canonical. When it fails but splitting on "/" yields more than three segments, we assume
// extra slashes came only from ref (and optionally a single logical path segment at the end),
// matching the common case for curl-style queries.
func NormalizeQueryDecodedBundleKey(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("bundle_key is empty")
	}
	if _, _, _, err := Parse(s); err == nil {
		return s, nil
	}
	parts := strings.Split(s, "/")
	if len(parts) < 3 {
		return "", fmt.Errorf("bundle_key must be repository/ref/path (got %q)", s)
	}
	if len(parts) == 3 {
		return "", fmt.Errorf("bundle_key is not a valid bundle key (got %q)", s)
	}
	// len >= 4: likely "%2F" in ref (and/or path) was decoded to "/".
	repo := parts[0]
	last := parts[len(parts)-1]
	ref := strings.Join(parts[1:len(parts)-1], "/")
	if last == "." {
		return Build(repo, ref, ""), nil
	}
	return Build(repo, ref, last), nil
}
