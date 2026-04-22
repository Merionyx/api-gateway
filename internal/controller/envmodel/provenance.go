package envmodel

import "github.com/merionyx/api-gateway/internal/controller/domain/models"

// StaticConfigSource names which input layer’s entry won for a static bundle key in the effective
// list (order: etcd (gRPC) over in-memory, k8s over file within in-memory). See ADR 0001.
type StaticConfigSource int

const (
	StaticConfigUnspecified StaticConfigSource = iota
	StaticConfigFile
	StaticConfigKubernetes
	StaticConfigEtcdGRPC
)

// ConfigSourceForStaticBundle reports the winning source for the given bundle in the already-merged
// effective list. Slices are unmerged static lists from file, K8s, and etcd (may be nil/empty).
func ConfigSourceForStaticBundle(eff models.StaticContractBundleConfig, file, k8s, etcd []models.StaticContractBundleConfig) StaticConfigSource {
	k := BundleKey(eff)
	etcdIdx := indexByBundleKey(etcd)
	if _, ok := etcdIdx[k]; ok {
		return StaticConfigEtcdGRPC
	}
	if _, ok := indexByBundleKey(k8s)[k]; ok {
		return StaticConfigKubernetes
	}
	if _, ok := indexByBundleKey(file)[k]; ok {
		return StaticConfigFile
	}
	return StaticConfigUnspecified
}

func indexByBundleKey(s []models.StaticContractBundleConfig) map[string]struct{} {
	if len(s) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(s))
	for _, b := range s {
		m[BundleKey(b)] = struct{}{}
	}
	return m
}
