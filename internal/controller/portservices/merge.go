package portservices

import (
	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

// MergeEnvStaticWithRootPoolUpstreams builds name → upstream: every line in the effective environment
// wins; then the root pool (file `services.static` then K8s globals not already in file) adds
// only names that are not yet present. This matches the intended policy of [xdsBuilder.BuildClusters]
// and the registry’s static service list.
func MergeEnvStaticWithRootPoolUpstreams(env *models.Environment, pool interfaces.InMemoryServiceRepository) map[string]string {
	out := make(map[string]string)
	if env != nil && env.Services != nil {
		for i := range env.Services.Static {
			s := env.Services.Static[i]
			out[s.Name] = s.Upstream
		}
	}
	if pool == nil {
		return out
	}
	fromFile, fromKube := pool.ListRootPoolDeduplicated()
	for i := range fromFile {
		s := fromFile[i]
		if _, ok := out[s.Name]; !ok {
			out[s.Name] = s.Upstream
		}
	}
	for i := range fromKube {
		s := fromKube[i]
		if _, ok := out[s.Name]; !ok {
			out[s.Name] = s.Upstream
		}
	}
	return out
}

// RootPoolDeduplicatedExcludingNames returns [ListRootPoolDeduplicated] slices restricted to
// service names that are not in `exclude` (e.g. names already present on the environment).
func RootPoolDeduplicatedExcludingNames(pool interfaces.InMemoryServiceRepository, exclude map[string]struct{}) (file, kube []models.StaticServiceConfig) {
	if pool == nil {
		return nil, nil
	}
	f, k := pool.ListRootPoolDeduplicated()
	for i := range f {
		if _, skip := exclude[f[i].Name]; !skip {
			file = append(file, f[i])
		}
	}
	for i := range k {
		if _, skip := exclude[k[i].Name]; !skip {
			kube = append(kube, k[i])
		}
	}
	return
}
