# api-gateway-control-plane

Helm chart for **Gateway Controller**, **API Server**, and **Contract Syncer**. Each component can be disabled via `components.*.enabled`.

## Prerequisites

- Kubernetes 1.21+
- **etcd** is not installed by this chart. Set `etcd.endpoints` and, if TLS is used, create a Secret with client certs and set `etcd.tls.existingSecret`.
- **Same namespace**: all referenced Secrets (`etcd.tls.existingSecret`, JWT, contract syncer, TLS mounts) must exist in **the Helm release namespace** (`helm install -n ...`). Kubernetes does not mount Secrets from other namespaces; copy the Secret or sync it (e.g. `kubectl get secret ... -n other | sed … | kubectl apply -n api-gateway -f -`).
- **TLS for gRPC**: either pre-create Secrets (see `tls.*` in `values.yaml`) or enable `certManager.createCertificates` and point `certManager.issuerRef` to a **ClusterIssuer** / **Issuer** that can issue both **server** and **client** certificates.
- **Internal CA** Secret (`tls.internalCASecret`, default name `{{ release }}-grpc-internal-ca-tls`): trust anchor at `/etc/grpc-internal-ca`. If you use `certManager.createCertificates` **without** `certManager.useNamespaceCAChain`, this Secret is **not** created automatically — copy `ca.crt` from a leaf Secret into a new Secret as `tls.crt`, or enable **`certManager.useNamespaceCAChain: true`** (CA Certificate + namespaced CA Issuer + leaf certs, same idea as `deployments/dev/.../grpc-certificates.yaml`).
- **JWT keys** for API Server: create a Secret (or set `components.apiServer.jwt.existingSecret`) with the expected key material under the mount path used by the application.
- **Contract Syncer** Git credentials: either one Secret via `components.contractSyncer.envFromSecret` (keys become env vars, e.g. `GITHUB_ACCESS_TOKEN`), or per-repo Secrets via `components.contractSyncer.repositoryTokens` (`envName` / `secretName` / `secretKey` — `envName` must match `token_env` in `components.contractSyncer.config.repositories`); do not put token values in `values.yaml`.

## Install

Default namespace reference: `api-gateway` (use `helm install -n api-gateway --create-namespace`).

```bash
helm install agw ./deployments/helm/api-gateway-control-plane -n api-gateway \
  --set 'etcd.endpoints[0]=https://etcd.example:2379' \
  --set etcd.tls.existingSecret=my-etcd-tls \
  --set components.apiServer.jwt.existingSecret=my-jwt-secret
```

## High availability

Increase `components.*.replicaCount`, enable PDBs (`*.pdb.enabled`), and use `defaultPodAntiAffinity` / `defaultTopologySpreadConstraints` (on by default when replicas &gt; 1).

## Multiple releases in one cluster

`ClusterRole` / `ClusterRoleBinding` names include the release fullname to avoid clashes between releases.
