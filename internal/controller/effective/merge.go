package effective

import (
	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/envmodel"
)

// ErrNotFound is returned by [MergeMemoryAndControllerEtcd] when neither the in-memory
// (file ∪ Kubernetes) view nor the etcd-stored environment is present for the name being resolved.
// Compare with [errors.Is](…, ErrNotFound) at call sites.
var ErrNotFound = envmodel.ErrBuildEffectiveNotFound

// MergeMemoryAndControllerEtcd is the ADR 0001 composition step: merge the in-memory
// environment (file ∪ K8s, already coalesced per name) with the optional gRPC/CRUD
// [models.Environment] in controller-local etcd. Implementation: [envmodel.BuildOptionalEffectiveEnvironment].
//
// Callers that need the lower-level building blocks (union rules, copy semantics) use
// [github.com/merionyx/api-gateway/internal/controller/envmodel] directly; this function is
// the stable, named façade for the control-plane data flow described in the package doc.
func MergeMemoryAndControllerEtcd(inMemory, controllerEtcd *models.Environment) (*models.Environment, error) {
	return envmodel.BuildOptionalEffectiveEnvironment(inMemory, controllerEtcd)
}
