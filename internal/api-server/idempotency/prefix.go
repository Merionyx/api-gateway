package idempotency

import (
	"strings"
)

// ResolveKeyPrefix builds the etcd key prefix for idempotency records (before /keys/… inside NewEtcdStore).
// base is usually from config idempotency.etcd_key_prefix; cluster isolates keys when multiple logical environments share one etcd.
func ResolveKeyPrefix(base, cluster string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "/api-gateway/api-server/idempotency/v1"
	}
	base = strings.TrimSuffix(base, "/")
	cluster = sanitizeClusterSegment(cluster)
	if cluster != "" {
		return base + "/clusters/" + cluster
	}
	return base
}

func sanitizeClusterSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "..", "_")
	return s
}
