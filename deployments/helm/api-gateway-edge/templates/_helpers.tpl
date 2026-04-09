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

{{- define "agwedge.secret.internalCA" -}}
{{- if .Values.tls.internalCASecret -}}
{{- .Values.tls.internalCASecret -}}
{{- else -}}
{{- printf "%s-grpc-internal-ca-tls" (include "agwedge.fullname" .) -}}
{{- end -}}
{{- end }}

{{- define "agwedge.secret.authSidecarGrpc" -}}
{{- if .Values.tls.authSidecarGrpcServerSecret -}}
{{- .Values.tls.authSidecarGrpcServerSecret -}}
{{- else -}}
{{- printf "%s-auth-sidecar-grpc-tls" (include "agwedge.fullname" .) -}}
{{- end -}}
{{- end }}

{{- define "agwedge.secret.grpcClientAuthSidecar" -}}
{{- if .Values.tls.grpcClientAuthSidecarSecret -}}
{{- .Values.tls.grpcClientAuthSidecarSecret -}}
{{- else -}}
{{- printf "%s-grpc-client-auth-sidecar-tls" (include "agwedge.fullname" .) -}}
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

{{- define "agwedge.auth.config.merged" -}}
{{- $def := fromYaml (include "agwedge.auth.config.defaults" .) -}}
{{- $usr := .Values.authSidecar.config | default dict -}}
{{- toYaml (mergeOverwrite $def $usr) -}}
{{- end }}

{{/* Envoy bootstrap file (ConfigMap data key envoy.xds-tls.yaml); used for checksum + configmap. */}}
{{- define "agwedge.envoy.bootstrapYaml" -}}
node:
  id: {{ default (printf "%s-envoy" (include "agwedge.fullname" .)) .Values.connectivity.envoyNodeId | quote }}
  cluster: {{ default .Values.connectivity.tenant .Values.connectivity.envoyCluster | quote }}

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
{{- end }}
