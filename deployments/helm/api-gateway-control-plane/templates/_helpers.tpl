{{- define "agwcp.name" -}}
{{- default "api-gateway" .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{- define "agwcp.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "agwcp.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end }}

{{- define "agwcp.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end }}

{{- define "agwcp.labels" -}}
helm.sh/chart: {{ include "agwcp.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/part-of: api-gateway
{{- end }}

{{- define "agwcp.componentSlug" -}}
{{- $c := index . 0 -}}
{{- if eq $c "apiServer" -}}api-server
{{- else if eq $c "contractSyncer" -}}contract-syncer
{{- else -}}{{- $c -}}
{{- end -}}
{{- end }}

{{/*
  Full image ref repository:tag. If image.tag is empty, uses Chart.AppVersion (set via helm package --app-version on release).
*/}}
{{- define "agwcp.imageRef" -}}
{{- $root := index . 0 -}}
{{- $comp := index . 1 -}}
{{- $cfg := index $root.Values.components $comp -}}
{{- $tag := $cfg.image.tag | default $root.Chart.AppVersion -}}
{{- printf "%s:%s" $cfg.image.repository $tag -}}
{{- end }}

{{- define "agwcp.componentLabels" -}}
{{- $component := index . 0 -}}
{{- $root := index . 1 -}}
{{- $slug := include "agwcp.componentSlug" (list $component) -}}
{{ include "agwcp.labels" $root }}
app.kubernetes.io/name: {{ include "agwcp.fullname" $root }}-{{ $slug }}
app.kubernetes.io/component: {{ $slug }}
{{- end }}

{{- define "agwcp.selectorLabels" -}}
{{- $component := index . 0 -}}
{{- $root := index . 1 -}}
{{- $slug := include "agwcp.componentSlug" (list $component) -}}
app.kubernetes.io/instance: {{ $root.Release.Name }}
app.kubernetes.io/component: {{ $slug }}
app.kubernetes.io/name: {{ include "agwcp.fullname" $root }}-{{ $slug }}
{{- end }}

{{- define "agwcp.serviceAccountName" -}}
{{- printf "%s-controller" (include "agwcp.fullname" .) -}}
{{- end }}

{{- define "agwcp.svc.controller" -}}
{{- printf "%s-controller" (include "agwcp.fullname" .) -}}
{{- end }}
{{- define "agwcp.svc.apiServer" -}}
{{- printf "%s-api-server" (include "agwcp.fullname" .) -}}
{{- end }}
{{- define "agwcp.svc.contractSyncer" -}}
{{- printf "%s-contract-syncer" (include "agwcp.fullname" .) -}}
{{- end }}

{{- define "agwcp.dns.apiServer" -}}
{{- printf "%s.%s.svc.cluster.local" (include "agwcp.svc.apiServer" .) .Release.Namespace -}}
{{- end }}
{{- define "agwcp.dns.contractSyncer" -}}
{{- printf "%s.%s.svc.cluster.local" (include "agwcp.svc.contractSyncer" .) .Release.Namespace -}}
{{- end }}
{{- define "agwcp.dns.controller" -}}
{{- printf "%s.%s.svc.cluster.local" (include "agwcp.svc.controller" .) .Release.Namespace -}}
{{- end }}

{{- define "agwcp.secret.grpcInternalCA" -}}
{{- if .Values.tls.internalCASecret -}}
{{- .Values.tls.internalCASecret -}}
{{- else -}}
{{- printf "%s-grpc-internal-ca-tls" (include "agwcp.fullname" .) -}}
{{- end -}}
{{- end }}

{{- define "agwcp.secret.controllerGrpcServer" -}}
{{- if .Values.tls.controller.grpcServerSecret -}}
{{- .Values.tls.controller.grpcServerSecret -}}
{{- else -}}
{{- printf "%s-controller-grpc-tls" (include "agwcp.fullname" .) -}}
{{- end -}}
{{- end }}

{{- define "agwcp.secret.grpcClientController" -}}
{{- if .Values.tls.controller.clientToApiServerSecret -}}
{{- .Values.tls.controller.clientToApiServerSecret -}}
{{- else -}}
{{- printf "%s-grpc-client-controller-tls" (include "agwcp.fullname" .) -}}
{{- end -}}
{{- end }}

{{- define "agwcp.secret.apiServerGrpcServer" -}}
{{- if .Values.tls.apiServer.grpcServerSecret -}}
{{- .Values.tls.apiServer.grpcServerSecret -}}
{{- else -}}
{{- printf "%s-api-server-grpc-tls" (include "agwcp.fullname" .) -}}
{{- end -}}
{{- end }}

{{- define "agwcp.secret.grpcClientApiServer" -}}
{{- if .Values.tls.apiServer.clientToContractSyncerSecret -}}
{{- .Values.tls.apiServer.clientToContractSyncerSecret -}}
{{- else -}}
{{- printf "%s-grpc-client-api-server-tls" (include "agwcp.fullname" .) -}}
{{- end -}}
{{- end }}

{{- define "agwcp.secret.contractSyncerGrpcServer" -}}
{{- if .Values.tls.contractSyncer.grpcServerSecret -}}
{{- .Values.tls.contractSyncer.grpcServerSecret -}}
{{- else -}}
{{- printf "%s-contract-syncer-grpc-tls" (include "agwcp.fullname" .) -}}
{{- end -}}
{{- end }}

{{/* API profile JWT signing keys (*.key PEM). */}}
{{- define "agwcp.secret.jwtApiKeys" -}}
{{- $a := trim (.Values.components.apiServer.jwt.apiKeysSecret | default "") -}}
{{- if ne $a "" -}}{{- $a -}}{{- else -}}{{- printf "%s-jwt-api-keys" (include "agwcp.fullname" .) -}}{{- end -}}
{{- end }}

{{/* Edge profile JWT signing keys (*.key PEM). */}}
{{- define "agwcp.secret.jwtEdgeKeys" -}}
{{- $e := trim (.Values.components.apiServer.jwt.edgeKeysSecret | default "") -}}
{{- if ne $e "" -}}{{- $e -}}{{- else -}}{{- printf "%s-jwt-edge-keys" (include "agwcp.fullname" .) -}}{{- end -}}
{{- end }}

{{- define "agwcp.defaultAntiAffinity" -}}
{{- $component := index . 0 -}}
{{- $root := index . 1 -}}
{{- $cfg := index $root.Values.components $component -}}
{{- if and $root.Values.defaultPodAntiAffinity (gt (int ($cfg.replicaCount | default 1)) 1) }}
podAntiAffinity:
  preferredDuringSchedulingIgnoredDuringExecution:
  - weight: 100
    podAffinityTerm:
      labelSelector:
        matchLabels:
          {{- include "agwcp.selectorLabels" (list $component $root) | nindent 10 }}
      topologyKey: kubernetes.io/hostname
{{- end }}
{{- end }}

{{- define "agwcp.defaultTopologySpread" -}}
{{- $component := index . 0 -}}
{{- $root := index . 1 -}}
{{- $cfg := index $root.Values.components $component -}}
{{- if and $root.Values.defaultTopologySpreadConstraints (gt (int ($cfg.replicaCount | default 1)) 1) }}
- maxSkew: 1
  topologyKey: topology.kubernetes.io/zone
  whenUnsatisfiable: ScheduleAnyway
  labelSelector:
    matchLabels:
      {{- include "agwcp.selectorLabels" (list $component $root) | nindent 6 }}
{{- end }}
{{- end }}

{{- define "agwcp.controller.config.defaults" -}}
server:
  http1_port: "8080"
  http2_port: "8443"
  grpc_port: "19090"
  xds_port: "19091"
  host: "0.0.0.0"
etcd:
  endpoints:
{{- range .Values.etcd.endpoints }}
  - {{ . | quote }}
{{- end }}
  dial_timeout: "5s"
  tls:
    enabled: true
    cert_file: "/etc/etcd-tls/tls.crt"
    key_file: "/etc/etcd-tls/tls.key"
    ca_file: "/etc/etcd-tls/ca.crt"
tenant: {{ .Values.components.controller.tenant | quote }}
ha:
  controller_id: {{ printf "%s-controller" (include "agwcp.fullname" .) | quote }}
leader_election:
  enabled: true
  key_prefix: {{ printf "/api-gateway/controller/election/%s" .Release.Name | quote }}
  identity: ""
  session_ttl_seconds: 5
api_server:
  address: {{ printf "%s:19093" (include "agwcp.svc.apiServer" .) | quote }}
grpc_control_plane:
  tls:
    enabled: true
    cert_file: /etc/grpc-control-plane-tls/tls.crt
    key_file: /etc/grpc-control-plane-tls/tls.key
    client_ca_file: /etc/grpc-internal-ca/tls.crt
  observability:
    reflection_enabled: true
    log_requests: false
grpc_xds:
  tls:
    enabled: true
    cert_file: /etc/grpc-control-plane-tls/tls.crt
    key_file: /etc/grpc-control-plane-tls/tls.key
    client_ca_file: /etc/grpc-internal-ca/tls.crt
  observability:
    reflection_enabled: true
    log_requests: false
grpc_api_server_client:
  enabled: true
  ca_file: /etc/grpc-internal-ca/tls.crt
  cert_file: /etc/grpc-client-api-server/tls.crt
  key_file: /etc/grpc-client-api-server/tls.key
  server_name: {{ include "agwcp.dns.apiServer" . | quote }}
metrics_http:
  enabled: true
  host: "0.0.0.0"
  port: "9090"
  path: "/metrics"
environments: []
services:
  static: []
kubernetes_discovery:
  enabled: {{ .Values.components.controller.kubernetesDiscovery.enabled }}
  resource_label_selector:
{{- range $k, $v := .Values.components.controller.kubernetesDiscovery.resourceLabelSelector }}
    {{ $k }}: {{ $v | quote }}
{{- end }}
{{- end }}

{{- define "agwcp.apiServer.config.defaults" -}}
server:
  http_port: "8080"
  grpc_port: "19093"
  host: "0.0.0.0"
  cors:
    allow_origins: []
etcd:
  endpoints:
{{- range .Values.etcd.endpoints }}
  - {{ . | quote }}
{{- end }}
  dial_timeout: "5s"
  tls:
    enabled: true
    cert_file: "/etc/etcd-tls/tls.crt"
    key_file: "/etc/etcd-tls/tls.key"
    ca_file: "/etc/etcd-tls/ca.crt"
jwt:
  keys_dir: "/api-server/secrets/api-server/keys/jwt"
  issuer: "api-gateway-api-server"
contract_syncer:
  address: {{ printf "%s:19092" (include "agwcp.svc.contractSyncer" .) | quote }}
readiness:
  require_contract_syncer: false
leader_election:
  enabled: true
  key_prefix: {{ printf "/api-gateway/api-server/election/%s" .Release.Name | quote }}
  identity: ""
  session_ttl_seconds: 5
grpc_registry:
  tls:
    enabled: true
    cert_file: /etc/grpc-server-tls/tls.crt
    key_file: /etc/grpc-server-tls/tls.key
    client_ca_file: /etc/grpc-internal-ca/tls.crt
  observability:
    reflection_enabled: true
    log_requests: false
grpc_contract_syncer_client:
  enabled: true
  ca_file: /etc/grpc-internal-ca/tls.crt
  cert_file: /etc/grpc-client-contract-syncer/tls.crt
  key_file: /etc/grpc-client-contract-syncer/tls.key
  server_name: {{ include "agwcp.dns.contractSyncer" . | quote }}
metrics_http:
  enabled: true
  host: "0.0.0.0"
  port: "9090"
  path: "/metrics"
idempotency:
  backend: "memory"
  bundle_sync_ttl: "24h"
  etcd_key_prefix: "/api-gateway/api-server/idempotency/v1"
  cluster: ""
auth:
  etcd_key_prefix: "/api-gateway/api-server/auth/v1"
  environment: "production"
  allow_insecure_bootstrap: false
{{- end }}

{{- define "agwcp.contractSyncer.config.defaults" -}}
server:
  grpc_port: "19092"
  host: "0.0.0.0"
  grpc:
    tls:
      enabled: true
      cert_file: /etc/grpc-server-tls/tls.crt
      key_file: /etc/grpc-server-tls/tls.key
      client_ca_file: /etc/grpc-internal-ca/tls.crt
    observability:
      reflection_enabled: true
      log_requests: false
metrics_http:
  enabled: true
  host: "0.0.0.0"
  port: "9090"
  path: "/metrics"
repositories: []
api_server:
  address: {{ printf "%s:19093" (include "agwcp.svc.apiServer" .) | quote }}
{{- end }}

{{/*
  Global .telemetry merged with .components.<name>.telemetry (component keys win).
  Shapes the same keys as internal/shared/telemetry FileBlock in repo YAML.
*/}}
{{- define "agwcp.telemetry.merged" -}}
{{- $root := index . 0 -}}
{{- $compName := index . 1 -}}
{{- $g := $root.Values.telemetry | default dict -}}
{{- $c := index $root.Values.components $compName -}}
{{- $p := $c.telemetry | default dict -}}
{{- toYaml (mergeOverwrite (mergeOverwrite (dict) $g) $p) -}}
{{- end }}

{{- define "agwcp.controller.config.merged" -}}
{{- $def := fromYaml (include "agwcp.controller.config.defaults" .) -}}
{{- $tel := fromYaml (include "agwcp.telemetry.merged" (list . "controller")) -}}
{{- $withTel := mergeOverwrite $def (dict "telemetry" $tel) -}}
{{- $usr := .Values.components.controller.config | default dict -}}
{{- toYaml (mergeOverwrite $withTel $usr) -}}
{{- end }}

{{- define "agwcp.apiServer.config.merged" -}}
{{- $def := fromYaml (include "agwcp.apiServer.config.defaults" .) -}}
{{- $tel := fromYaml (include "agwcp.telemetry.merged" (list . "apiServer")) -}}
{{- $withTel := mergeOverwrite $def (dict "telemetry" $tel) -}}
{{- $usr := .Values.components.apiServer.config | default dict -}}
{{- $merged := mergeOverwrite $withTel $usr -}}
{{- $usrJwt := index $usr "jwt" | default dict -}}
{{- $explicitEdgeDir := trim (toString (index $usrJwt "edge_keys_dir" | default "")) -}}
{{- $jwtAutoEdge := "/api-server/secrets/api-server/keys/jwt-edge" -}}
{{- $jwtNow := index $merged "jwt" | default dict -}}
{{- if eq $explicitEdgeDir "" -}}
{{- $merged = mergeOverwrite $merged (dict "jwt" (mergeOverwrite $jwtNow (dict "edge_keys_dir" $jwtAutoEdge))) -}}
{{- end -}}
{{- toYaml $merged -}}
{{- end }}

{{- define "agwcp.contractSyncer.config.merged" -}}
{{- $def := fromYaml (include "agwcp.contractSyncer.config.defaults" .) -}}
{{- $tel := fromYaml (include "agwcp.telemetry.merged" (list . "contractSyncer")) -}}
{{- $withTel := mergeOverwrite $def (dict "telemetry" $tel) -}}
{{- $usr := .Values.components.contractSyncer.config | default dict -}}
{{- toYaml (mergeOverwrite $withTel $usr) -}}
{{- end }}

{{/*
  OIDC provider id → env suffix for AGWCP_OIDC_<SUFFIX>_CLIENT_ID (must match internal/api-server/config/auth_oidc_env.go).
*/}}
{{- define "agwcp.oidcEnvSuffix" -}}
{{- $id := toString . -}}
{{- $s := regexReplaceAll "[^a-zA-Z0-9]+" $id "_" -}}
{{- upper (trimAll "_" $s) -}}
{{- end }}
