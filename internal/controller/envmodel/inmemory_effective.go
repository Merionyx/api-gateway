package envmodel

import "github.com/merionyx/api-gateway/internal/controller/domain/models"

// InMemoryEffective returns the in-memory (file ∪ Kubernetes) view: merge when both partials
// exist, otherwise a skeleton copy of the single source. It never returns a shared *Environment
// that aliases repository storage, so call sites cannot corrupt file/K8s snapshots. Nil when both
// are nil.
func InMemoryEffective(file, k8s *models.Environment) *models.Environment {
	if file == nil && k8s == nil {
		return nil
	}
	if file != nil && k8s != nil {
		return MergeFileAndK8s(file, k8s)
	}
	if k8s != nil {
		return ToAPIServerSkeleton(k8s)
	}
	return ToAPIServerSkeleton(file)
}
