# api-gateway-edge

Helm chart for **Envoy** and **Auth Sidecar** (single Deployment, two containers). Install once per edge pool; use a separate release from `api-gateway-control-plane`.

## Prerequisites

- Install **api-gateway-control-plane** first (with `certManager.useNamespaceCAChain` so the namespaced **Issuer** `…-grpc-ca-issuer` and CA Secret `…-grpc-internal-ca-tls` exist).
- Set **connectivity** to your control plane (plain strings): `controllerGrpcAddress`, `controllerServerName`, `jwksUrl`, `environment`, and xDS settings `xdsHost`, `xdsPort`, `xdsSni`.
- **TLS**: `tls.internalCASecret` must reference the **same** CA Secret as the control-plane release. For cert-manager–issued edge certs, set `certManager.createCertificates: true` and `certManager.issuerRef` to **`kind: Issuer`**, **`name: <cp-release>-api-gateway-control-plane-grpc-ca-issuer`** (same namespace as edge) so Envoy/auth-sidecar certs chain to the same CA as the controller (mTLS).
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
