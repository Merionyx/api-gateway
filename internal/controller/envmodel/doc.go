// Package envmodel implements the static merge and fingerprint rules for Gateway Controller
// environments: combining file- and Kubernetes-sourced config, optional union with etcd-stored
// static data, and helpers for API Server sync (provenance, skeleton copies).
//
// The end-to-end ADR 0001 story (what calls what) lives in
// [github.com/merionyx/api-gateway/internal/controller/effective]. Start there; this package is
// the implementation layer for merge algorithms and static analysis.
package envmodel
