// Package effective is the single entry for understanding how ADR 0001 is wired in the Gateway
// Controller: static layers → effective environment → xDS and optional materialized state.
//
// # Pipeline (order of steps)
//
//  1. In-process static inputs are merged per environment name: configuration loaded from
//  local file and from Kubernetes (Custom Resources) is combined into one in-memory
//  [github.com/merionyx/api-gateway/internal/controller/domain/models.Environment] per name
//  (see [github.com/merionyx/api-gateway/internal/controller/repository/memory] and
//  [github.com/merionyx/api-gateway/internal/controller/envmodel.InMemoryEffective]).
//
//  2. That in-memory view is merged with the optional copy of the same name stored in
//  controller-local etcd (gRPC CRUD). The public entry point for this merge is
//  [MergeMemoryAndControllerEtcd]; the algorithm lives in [github.com/merionyx/api-gateway/internal/controller/envmodel]
//  ([github.com/merionyx/api-gateway/internal/controller/envmodel.BuildOptionalEffectiveEnvironment]).
//
//  3. The reconciler enriches the result with contract snapshots, builds the Envoy xDS snapshot,
//  and — when the process is leader and materialization is enabled — updates the
//  materialized effective document (see [github.com/merionyx/api-gateway/internal/controller/reconcile]).
//
// # Diagram (Mermaid, for renderers that support it)
//
//	flowchart LR
//	    subgraph static["Per env name"]
//	        F[file config] --> M[InMemoryEffective]
//	        K[Kubernetes] --> M
//	    end
//	    M --> B[MergeMemoryAndControllerEtcd / envmodel]
//	    E[(controller etcd gRPC state)] --> B
//	    B --> R[reconcile.Reconciler]
//	    R --> X[xDS snapshot]
//	    R --> T[(optional materialized /effective/…)]
//
// # References
//
//	- ADR: docs/adr/0001-effective-environments-and-materialized-state.md
//	- Short overview: docs/architecture-environments.md
//	- Сценарные тесты merge+reconcile (in-memory, etcd, xDS, materialized): пакет reconcile, файл
//	  adr_merge_reconcile_test.go (см. бэклог п.8 / gateway-controller).
package effective
