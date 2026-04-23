# api-gateway-edge

Helm chart for **Envoy** and **Sidecar** (single Deployment, two containers). Install once per edge pool; use a separate release from `api-gateway-control-plane`.

## Prerequisites

- Install **api-gateway-control-plane** first (with `certManager.useNamespaceCAChain` so the namespaced **Issuer** `…-grpc-ca-issuer` and CA Secret `…-grpc-internal-ca-tls` exist).
- Set **connectivity** to your control plane (plain strings): `controllerGrpcAddress`, `controllerServerName`, `jwksUrl`, `environment`, and xDS settings `xdsHost`, `xdsPort`, `xdsSni`.
- **TLS**: `tls.internalCASecret` must reference the **same** CA Secret as the control-plane release. For cert-manager–issued edge certs, set `certManager.createCertificates: true` and `certManager.issuerRef` to **`kind: Issuer`**, **`name: <cp-release>-api-gateway-control-plane-grpc-ca-issuer`** (same namespace as edge) so Envoy/sidecar certs chain to the same CA as the controller (mTLS).
- **Cross-cluster**: use full hostnames / DNS that resolve from the edge cluster to API Server and Controller services; copy or sync TLS Secrets into the edge namespace if needed.

## Install

```bash
helm install edge ./deployments/helm/api-gateway-edge -n api-gateway \
  --set connectivity.controllerGrpcAddress=my-controller:19090 \
  --set connectivity.controllerServerName=my-controller.api-gateway.svc.cluster.local \
  --set connectivity.jwksUrl=http://my-api-server:8080/.well-known/jwks.json \
  --set connectivity.environment=prod \
  --set connectivity.xdsHost=my-controller \
  --set connectivity.xdsSni=my-controller.api-gateway.svc.cluster.local
```

## High availability

Set `replicaCount` &gt; 1 and keep `envoy.pdb.enabled` (PDB is created only when replicas &gt; 1).

## Tracing (Envoy + sidecar)

- **Auth sidecar** uses the top-level `telemetry` block in `values.yaml` (same key names as application configs: `enabled`, `service_name`, `otlp_endpoint`, …) with optional `sidecar.telemetry` overrides. This becomes `telemetry:` in the auth `ConfigMap`.
- **Envoy** uses `envoy.tracing`. When `envoy.tracing.enabled` is true, the chart injects a bootstrap [OpenTelemetry](https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/trace/v3/opentelemetry.proto.html) tracer (OTLP gRPC) and a static `clusters` entry for the collector. Defaults:
  - `envoy.tracing.openTelemetry` — `serviceName` (defaults to `api-gateway-edge-<connectivity.environment>`; if `connectivity.environment` is empty, `{{ release }}-api-gateway-edge-envoy`), `grpcTimeout`, `clusterName` (static cluster, defaults to `{{ release }}-api-gateway-edge-otlp-trace`)
  - `envoy.tracing.collector` — `host` / `port` for the OTLP gRPC socket; or leave `host` empty and set `inheritCollectorFromTelemetry: true` to copy host:port from `telemetry.otlp_endpoint` (`http://host:port` form).
- **HTTP connection manager (xDS from controller)** is configured with 100% client/random/overall sampling and `spawn_upstream_span: true` for edge; adjust that behavior in the controller if you need a different global policy. Fine-grained Envoy-only tuning (e.g. runtime sampling) can be appended with `envoy.tracing.appendBootstrap` (raw YAML, additional **root-level** keys only; avoid clobbering `tracing` / `static_resources` unless you replace them intentionally).
- Pointing **Envoy** and **auth** to different OTLP hosts is supported: set a dedicated `envoy.tracing.collector` while keeping `telemetry` for the sidecar.
