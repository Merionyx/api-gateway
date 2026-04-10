package cache

import (
	"strings"

	"github.com/merionyx/api-gateway/internal/controller/repository/etcd"
)

// ParseEnvironmentNameFromConfigKey parses /api-gateway/controller/environments/{name}/config.
func ParseEnvironmentNameFromConfigKey(key string) (name string, ok bool) {
	p := etcd.EnvironmentPrefix
	if !strings.HasPrefix(key, p) || !strings.HasSuffix(key, "/config") {
		return "", false
	}
	mid := strings.TrimSuffix(strings.TrimPrefix(key, p), "/config")
	if mid == "" || strings.Contains(mid, "/") {
		return "", false
	}
	return mid, true
}
