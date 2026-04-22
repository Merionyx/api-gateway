package envmodel

import "github.com/merionyx/api-gateway/internal/controller/domain/models"

// MergeInMemoryWithEtcd is a building block of [BuildOptionalEffectiveEnvironment] when both
// sides exist. Union: etcd wins on bundle key / service name; name and type from mem. Snapshots cleared.
// mem must be non-nil; etcd may be only partial (Static* slices).
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
