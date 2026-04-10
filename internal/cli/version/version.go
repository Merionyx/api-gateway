// Package version holds build metadata for agwctl (overridable via -ldflags).
package version

import (
	"fmt"
	"runtime"
	"strings"
)

// Overridden at link time, e.g.:
//
//	go build -ldflags "-X github.com/merionyx/api-gateway/internal/cli/version.Version=1.2.3 \
//	  -X github.com/merionyx/api-gateway/internal/cli/version.Commit=$(git rev-parse --short HEAD) \
//	  -X github.com/merionyx/api-gateway/internal/cli/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var (
	Version   = "dev"
	Commit    = ""
	BuildTime = ""
)

// Line returns a single line for cobra's --version (e.g. "1.0.0 (abc1234)").
func Line() string {
	v := strings.TrimSpace(Version)
	if v == "" {
		v = "dev"
	}
	if c := strings.TrimSpace(Commit); c != "" {
		return fmt.Sprintf("%s (%s)", v, c)
	}
	return v
}

// Details returns multi-line output for `agwctl version`.
func Details() string {
	var b strings.Builder
	v := strings.TrimSpace(Version)
	if v == "" {
		v = "dev"
	}
	fmt.Fprintf(&b, "agwctl %s\n", v)
	if c := strings.TrimSpace(Commit); c != "" {
		fmt.Fprintf(&b, "commit: %s\n", c)
	}
	if t := strings.TrimSpace(BuildTime); t != "" {
		fmt.Fprintf(&b, "build: %s\n", t)
	}
	fmt.Fprintf(&b, "go: %s\n", runtime.Version())
	return strings.TrimSuffix(b.String(), "\n")
}
