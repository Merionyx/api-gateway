package envmodel

import (
	"errors"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

// ErrBuildEffectiveNotFound is returned by BuildOptionalEffectiveEnvironment when neither
// side declares the environment. Callers that treat "missing" as non-fatal can use errors.Is.
var ErrBuildEffectiveNotFound = errors.New("no environment from memory or etcd")

// BuildOptionalEffectiveEnvironment merges the in-memory view (file ∪ Kubernetes) with the
// etcd gRPC–stored environment. Returns ErrBuildEffectiveNotFound if both are nil.
func BuildOptionalEffectiveEnvironment(mem, etcd *models.Environment) (*models.Environment, error) {
	if mem == nil && etcd == nil {
		return nil, ErrBuildEffectiveNotFound
	}
	if mem == nil {
		return ToAPIServerSkeleton(etcd), nil
	}
	if etcd == nil {
		return ToAPIServerSkeleton(mem), nil
	}
	return MergeInMemoryWithEtcd(mem, etcd), nil
}
