package envmodel

import (
	"errors"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

// ErrBuildEffectiveNotFound is returned when both inputs are nil. The package
// github.com/merionyx/api-gateway/internal/controller/effective aliases it as ErrNotFound.
var ErrBuildEffectiveNotFound = errors.New("no environment from memory or etcd")

// BuildOptionalEffectiveEnvironment implements the merge used by [github.com/merionyx/api-gateway/internal/controller/effective.MergeMemoryAndControllerEtcd].
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
