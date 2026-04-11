// Package version holds link-time metadata for the API Server binary (ldflags) and cached OpenAPI info.version.
package version

import (
	"sync"

	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
)

// Set at link time (see build/release/Dockerfile).
var (
	Release       = "dev"
	GitRevision   = "unknown"
	BuildTime     = "unknown"
	apiSchemaOnce sync.Once
	apiSchema     string
)

// APISchemaVersion returns OpenAPI info.version from the embedded spec (same document as request validation).
func APISchemaVersion() string {
	apiSchemaOnce.Do(func() {
		sw, err := apiserver.GetSwagger()
		if err != nil || sw == nil || sw.Info == nil || sw.Info.Version == "" {
			apiSchema = "unknown"
			return
		}
		apiSchema = sw.Info.Version
	})
	return apiSchema
}
