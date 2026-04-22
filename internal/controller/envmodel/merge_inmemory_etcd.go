package envmodel

import "github.com/merionyx/api-gateway/internal/controller/domain/models"

// MergeInMemoryWithEtcd combines the in-memory (file ∪ Kubernetes) view with etcd-stored
// static bundles and services. Union rules: second argument wins on bundle key / service name;
// name and type come from mem. Snapshots are cleared in the result.
// mem must be non-nil; etcd may be used only in union via Static*Slice.
func MergeInMemoryWithEtcd(mem, etcd *models.Environment) *models.Environment {
	uB := UnionStaticBundles(StaticBundleSlice(mem), StaticBundleSlice(etcd))
	uS := UnionStaticServices(StaticServiceSlice(mem), StaticServiceSlice(etcd))
	return &models.Environment{
		Name:      mem.Name,
		Type:      mem.Type,
		Bundles:   &models.EnvironmentBundleConfig{Static: uB},
		Services:  &models.EnvironmentServiceConfig{Static: uS},
		Snapshots: nil,
	}
}
