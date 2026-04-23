{{- define "agwedge.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{- define "agwedge.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name (include "agwedge.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end }}

{{- define "agwedge.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" -}}
{{- end }}

{{- define "agwedge.labels" -}}
helm.sh/chart: {{ include "agwedge.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/part-of: api-gateway
app.kubernetes.io/component: edge
{{- end }}

{{- define "agwedge.selectorLabels" -}}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/name: {{ include "agwedge.fullname" . }}-edge
app.kubernetes.io/component: edge
{{- end }}

{{/*
  Merionyx sidecar image. Empty tag → Chart.AppVersion (like in api-gateway-control-plane).
  Envoy — separate stream; tag only from .Values.envoy.image.tag.
*/}}
{{- define "agwedge.sidecarImage" -}}
{{- $tag := .Values.sidecar.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" .Values.sidecar.image.repository $tag -}}
{{- end }}

{{- define "agwedge.secret.internalCA" -}}
{{- if .Values.tls.internalCASecret -}}
{{- .Values.tls.internalCASecret -}}
{{- else -}}
{{- printf "%s-grpc-internal-ca-tls" (include "agwedge.fullname" .) -}}
{{- end -}}
{{- end }}

{{- define "agwedge.secret.sidecarGrpc" -}}
{{- if .Values.tls.sidecarGrpcServerSecret -}}
{{- .Values.tls.sidecarGrpcServerSecret -}}
{{- else -}}
{{- printf "%s-sidecar-grpc-tls" (include "agwedge.fullname" .) -}}
{{- end -}}
{{- end }}

{{- define "agwedge.secret.grpcClientsidecar" -}}
{{- if .Values.tls.grpcClientsidecarSecret -}}
{{- .Values.tls.grpcClientsidecarSecret -}}
{{- else -}}
{{- printf "%s-grpc-client-sidecar-tls" (include "agwedge.fullname" .) -}}
{{- end -}}
{{- end }}

{{- define "agwedge.secret.grpcClientEnvoy" -}}
{{- if .Values.tls.grpcClientEnvoySecret -}}
{{- .Values.tls.grpcClientEnvoySecret -}}
{{- else -}}
{{- printf "%s-grpc-client-envoy-tls" (include "agwedge.fullname" .) -}}
{{- end -}}
{{- end }}

{{- define "agwedge.defaultAntiAffinity" -}}
{{- if and .Values.defaultPodAntiAffinity (gt (int .Values.replicaCount) 1) }}
podAntiAffinity:
  preferredDuringSchedulingIgnoredDuringExecution:
  - weight: 100
    podAffinityTerm:
      labelSelector:
        matchLabels:
          {{- include "agwedge.selectorLabels" . | nindent 10 }}
      topologyKey: kubernetes.io/hostname
{{- end }}
{{- end }}

{{- define "agwedge.defaultTopologySpread" -}}
{{- if and .Values.defaultTopologySpreadConstraints (gt (int .Values.replicaCount) 1) }}
- maxSkew: 1
  topologyKey: topology.kubernetes.io/zone
  whenUnsatisfiable: ScheduleAnyway
  labelSelector:
    matchLabels:
      {{- include "agwedge.selectorLabels" . | nindent 6 }}
{{- end }}
{{- end }}

{{- define "agwedge.auth.config.defaults" -}}
server:
  grpc_port: "9001"
  host: "0.0.0.0"
controller:
  address: {{ .Values.connectivity.controllerGrpcAddress | quote }}
  environment: {{ .Values.connectivity.environment | quote }}
jwt:
  jwks_url: {{ .Values.connectivity.jwksUrl | quote }}
grpc_ext_authz:
  tls:
    enabled: false
  observability:
    reflection_enabled: true
    log_requests: false
grpc_controller_client:
  enabled: true
  ca_file: /etc/grpc-internal-ca/tls.crt
  cert_file: /etc/grpc-client-controller/tls.crt
  key_file: /etc/grpc-client-controller/tls.key
  server_name: {{ .Values.connectivity.controllerServerName | quote }}
metrics_http:
  enabled: true
  host: "0.0.0.0"
  port: "9090"
  path: "/metrics"
{{- end }}

{{- define "agwedge.telemetry.merged" -}}
{{- $g := .Values.telemetry | default dict -}}
{{- $p := .Values.sidecar.telemetry | default dict -}}
{{- toYaml (mergeOverwrite (mergeOverwrite (dict) $g) $p) -}}
{{- end }}

{{- define "agwedge.auth.config.merged" -}}
{{- $def := fromYaml (include "agwedge.auth.config.defaults" .) -}}
{{- $tel := fromYaml (include "agwedge.telemetry.merged" .) -}}
{{- $withTel := mergeOverwrite $def (dict "telemetry" $tel) -}}
{{- $usr := .Values.sidecar.config | default dict -}}
{{- toYaml (mergeOverwrite $withTel $usr) -}}
{{- end }}

{{/* OTLP trace cluster name; referenced by OpenTelemetryConfig in bootstrap. */}}
{{- define "agwedge.tracing.clusterName" -}}
{{- default (printf "%s-otlp-trace" (include "agwedge.fullname" .)) .Values.envoy.tracing.openTelemetry.clusterName -}}
{{- end -}}

{{/*
  JSON: {"host":"..","port":4317} when .envoy.tracing.collector.host is set, or
  .telemetry.otlp_endpoint matches host:port and inherit is true. Empty if unresolved.
*/}}
{{- define "agwedge.tracing.parsed" -}}
{{- $e := .Values.envoy.tracing -}}
{{- if (default false $e.enabled) -}}
{{- $c := $e.collector | default dict -}}
{{- $h := (default "" $c.host) | toString | trim -}}
{{- if ne $h "" -}}
{{- dict "host" $h "port" (int (default 4317 $c.port)) | toJson -}}
{{- else if and (default true $e.inheritCollectorFromTelemetry) (default "" .Values.telemetry.otlp_endpoint) -}}
{{- $rest := .Values.telemetry.otlp_endpoint | replace "https://" "" | replace "http://" "" | trim -}}
{{- if (regexMatch `^[^:]+:[0-9]+$` $rest) -}}
{{- $pstr := regexFind `[0-9]+$` $rest -}}
{{- $h := regexReplaceAll `:[0-9]+$` $rest "" -}}
{{- dict "host" $h "port" (int $pstr) | toJson -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/* service.name in Envoy bootstrap OpenTelemetry (Jaeger, etc.): api-gateway-edge-<environment> by default. */}}
{{- define "agwedge.tracing.serviceName" -}}
{{- $env := (default "" .Values.connectivity.environment) | toString | trim -}}
{{- $def := ternary (printf "api-gateway-edge-%s" $env) (printf "%s-envoy" (include "agwedge.fullname" .)) (ne $env "") -}}
{{- default $def .Values.envoy.tracing.openTelemetry.serviceName -}}
{{- end -}}

{{/* Envoy bootstrap file (ConfigMap data key envoy.xds-tls.yaml); used for checksum + configmap. */}}
{{- define "agwedge.envoy.bootstrapYaml" -}}
node:
  id: {{ default (printf "%s-envoy" (include "agwedge.fullname" .)) .Values.connectivity.envoyNodeId | quote }}
  cluster: {{ default .Values.connectivity.tenant .Values.connectivity.envoyCluster | quote }}

{{ if and .Values.envoy.tracing.enabled (include "agwedge.tracing.parsed" . | trim) -}}
{{- $gto := .Values.envoy.tracing.openTelemetry.grpcTimeout | default "3s" -}}
tracing:
  http:
    name: envoy.tracers.opentelemetry
    typed_config:
      "@type": type.googleapis.com/envoy.config.trace.v3.OpenTelemetryConfig
      service_name: {{ include "agwedge.tracing.serviceName" . | quote }}
      grpc_service:
        envoy_grpc:
          cluster_name: {{ include "agwedge.tracing.clusterName" . | quote }}
        timeout: {{ $gto | quote }}
{{- end }}
admin:
  address:
    socket_address:
      address: 0.0.0.0
      port_value: 9901

dynamic_resources:
  ads_config:
    api_type: GRPC
    transport_api_version: V3
    grpc_services:
    - envoy_grpc:
        cluster_name: xds_cluster

  cds_config:
    resource_api_version: V3
    ads: {}

  lds_config:
    resource_api_version: V3
    ads: {}

static_resources:
  clusters:
  - name: xds_cluster
    type: STRICT_DNS
    connect_timeout: 1s
    lb_policy: ROUND_ROBIN
    common_lb_config:
      locality_weighted_lb_config: {}
    typed_extension_protocol_options:
      envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
        "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
        explicit_http_config:
          http2_protocol_options: {}
    transport_socket:
      name: envoy.transport_sockets.tls
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.UpstreamTlsContext
        sni: {{ .Values.connectivity.xdsSni | quote }}
        common_tls_context:
          tls_certificates:
          - certificate_chain:
              filename: /etc/grpc-xds-client-tls/tls.crt
            private_key:
              filename: /etc/grpc-xds-client-tls/tls.key
          validation_context:
            trusted_ca:
              filename: /etc/grpc-internal-ca/tls.crt
    load_assignment:
      cluster_name: xds_cluster
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: {{ .Values.connectivity.xdsHost | quote }}
                port_value: {{ int .Values.connectivity.xdsPort }}
{{ if and .Values.envoy.tracing.enabled (include "agwedge.tracing.parsed" . | trim) -}}
  {{ $tc := fromJson (include "agwedge.tracing.parsed" . | trim) }}
  - name: {{ include "agwedge.tracing.clusterName" . | quote }}
    type: STRICT_DNS
    connect_timeout: 1s
    lb_policy: ROUND_ROBIN
    common_lb_config:
      locality_weighted_lb_config: {}
    typed_extension_protocol_options:
      envoy.extensions.upstreams.http.v3.HttpProtocolOptions:
        "@type": type.googleapis.com/envoy.extensions.upstreams.http.v3.HttpProtocolOptions
        explicit_http_config:
          http2_protocol_options: {}
    load_assignment:
      cluster_name: {{ include "agwedge.tracing.clusterName" . | quote }}
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: {{ $tc.host | quote }}
                port_value: {{ int $tc.port }}
{{- end -}}
{{- with (trim (default "" .Values.envoy.tracing.appendBootstrap)) -}}
{{ . | nindent 0 }}
{{- end -}}
{{- end }}
