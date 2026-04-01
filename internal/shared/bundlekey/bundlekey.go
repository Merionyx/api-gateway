package bundlekey

import (
	"fmt"
	"strings"
)

// EscapeRef encodes slashes in a git ref so it forms one etcd path segment.
func EscapeRef(ref string) string {
	return strings.ReplaceAll(ref, "/", "%2F")
}

// EscapePath encodes a bundle root path for one etcd segment.
// Empty logical path becomes "." so repository/ref/path always has three segments.
func EscapePath(logicalPath string) string {
	if logicalPath == "" {
		return "."
	}
	return strings.ReplaceAll(logicalPath, "/", "%2F")
}

// Build returns repository/escapedRef/escapedPath (same convention as API Server snapshot keys).
func Build(repository, ref, logicalPath string) string {
	return fmt.Sprintf("%s/%s/%s", repository, EscapeRef(ref), EscapePath(logicalPath))
}

// Parse splits a key from Build into logical ref and path (decoded).
func Parse(bundleKey string) (repository, ref, logicalPath string, err error) {
	parts := strings.Split(bundleKey, "/")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("bundle key must be repository/ref/path (slashes in ref and path escaped as %%2F): got %q", bundleKey)
	}
	repository = parts[0]
	ref = strings.ReplaceAll(parts[1], "%2F", "/")
	switch parts[2] {
	case ".":
		logicalPath = ""
	default:
		logicalPath = strings.ReplaceAll(parts[2], "%2F", "/")
	}
	return repository, ref, logicalPath, nil
}
