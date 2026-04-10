package cache

import (
	"strings"

	"github.com/merionyx/api-gateway/internal/controller/repository/etcd"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
)

// ParseSchemaContractEtcdKey extracts repository, ref, and bundle path from a contract snapshot key.
// Expected: /api-gateway/controller/schemas/{repo}/{escapedRef}/{escapedPath}/contracts/{contract}/snapshot
func ParseSchemaContractEtcdKey(key string) (repository, ref, path string, ok bool) {
	p := strings.TrimPrefix(key, etcd.SchemaPrefix)
	if p == key {
		return "", "", "", false
	}
	p = strings.TrimPrefix(p, "/")
	parts := strings.Split(p, "/")
	// repo, escapedRef, escapedPath, "contracts", contractName, "snapshot"
	if len(parts) < 6 || parts[len(parts)-1] != "snapshot" || parts[len(parts)-3] != "contracts" {
		return "", "", "", false
	}
	repository = parts[0]
	ref = strings.ReplaceAll(parts[1], "%2F", "/")
	switch parts[2] {
	case ".":
		path = ""
	default:
		path = strings.ReplaceAll(parts[2], "%2F", "/")
	}
	return repository, ref, path, true
}

// BundleKeyFromSchemaEtcdKey returns bundlekey.Build(...) when ParseSchemaContractEtcdKey succeeds.
func BundleKeyFromSchemaEtcdKey(key string) (string, bool) {
	repo, ref, p, ok := ParseSchemaContractEtcdKey(key)
	if !ok {
		return "", false
	}
	return bundlekey.Build(repo, ref, p), true
}
