package cache

import (
	"strings"

	"github.com/merionyx/api-gateway/internal/controller/repository/etcd"
)

// ControllerEtcdKeyEffect describes how one etcd key under the controller prefix should be handled for xDS/auth watchers.
type ControllerEtcdKeyEffect struct {
	// Ignore is true for keys that should not trigger rebuild or notify (e.g. leader election).
	Ignore bool
	// SchemaBundleKey is set when the key is a contract snapshot under schemas/; invalidate list cache and map to envs via index.
	SchemaBundleKey string
	// Environment is set for /environments/{name}/config changes.
	Environment string
	// UnknownUnderPrefix is true for other keys under ControllerWatchPrefix (full reconcile).
	UnknownUnderPrefix bool
}

// ClassifyControllerEtcdWatchKey classifies a single key for debounced follower and auth etcd watches.
func ClassifyControllerEtcdWatchKey(key string) ControllerEtcdKeyEffect {
	if strings.HasPrefix(key, etcd.ControllerWatchPrefix+"election/") {
		return ControllerEtcdKeyEffect{Ignore: true}
	}
	if bk, ok := BundleKeyFromSchemaEtcdKey(key); ok {
		return ControllerEtcdKeyEffect{SchemaBundleKey: bk}
	}
	if envName, ok := ParseEnvironmentNameFromConfigKey(key); ok {
		return ControllerEtcdKeyEffect{Environment: envName}
	}
	if strings.HasPrefix(key, etcd.ControllerWatchPrefix) {
		return ControllerEtcdKeyEffect{UnknownUnderPrefix: true}
	}
	return ControllerEtcdKeyEffect{}
}
