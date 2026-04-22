// Package reconcile turns an effective [github.com/merionyx/api-gateway/internal/controller/domain/models.Environment]
// into an Envoy xDS snapshot and, when [MaterializedWritePolicy] allows, into a materialized
// effective document in etcd (ADR 0001). It composes: load layers → [github.com/merionyx/api-gateway/internal/controller/effective.MergeMemoryAndControllerEtcd]
// → schema snapshot list → [github.com/merionyx/api-gateway/internal/controller/xds/snapshot] → cache update.
//
// For the full pipeline description, read [github.com/merionyx/api-gateway/internal/controller/effective] first
// and ADR 0001.
package reconcile
