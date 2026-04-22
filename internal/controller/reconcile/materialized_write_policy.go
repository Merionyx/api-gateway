package reconcile

import (
	"reflect"

	"github.com/merionyx/api-gateway/internal/controller/domain/interfaces"
	"github.com/merionyx/api-gateway/internal/shared/election"
)

// MaterializedWritePolicy is the only gate for writing or deleting the **materialized effective**
// JSON under the controller’s etcd prefix (ADR 0001, docs/adr/0001-…). It composes: feature flag,
// non-nil store, and this replica being the elected leader.
type MaterializedWritePolicy struct {
	enabled bool
	store   interfaces.MaterializedEffectiveStore
	leader  election.LeaderGate
}

// NewMaterializedWritePolicy builds a policy. A nil store means no materialized I/O regardless of
// other fields.
func NewMaterializedWritePolicy(
	enabled bool,
	store interfaces.MaterializedEffectiveStore,
	leader election.LeaderGate,
) MaterializedWritePolicy {
	return MaterializedWritePolicy{enabled: enabled, store: store, leader: leader}
}

// Allow is true when this process may ReconcileIfChanged or Delete the materialized effective key.
func (p MaterializedWritePolicy) Allow() bool {
	if !p.enabled || isNilishMaterializedStore(p.store) {
		return false
	}
	if p.leader == nil {
		return false
	}
	return p.leader.IsLeader()
}

// isNilishMaterializedStore is true for nil and for typed nil pointers in the interface
// (e.g. *MaterializedStore)(nil) — a plain "== nil" is not enough in Go.
func isNilishMaterializedStore(s interfaces.MaterializedEffectiveStore) bool {
	if s == nil {
		return true
	}
	v := reflect.ValueOf(s)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.UnsafePointer,
		reflect.Ptr, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}
