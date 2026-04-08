package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// APIServer runs the JWT/API + gRPC control API.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=apiservers,shortName=merionyxapi
type APIServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   APIServerSpec   `json:"spec,omitempty"`
	Status APIServerStatus `json:"status,omitempty"`
}

type APIServerSpec struct {
	PodWorkloadSpec `json:",inline"`

	Server APIServerServerSpec `json:"server"`
	Etcd   EtcdSpec            `json:"etcd"`

	JWTKeysSecretRef corev1.LocalObjectReference `json:"jwtKeysSecretRef"`
	JWTIssuer        string                      `json:"jwtIssuer,omitempty"`

	ContractSyncerAddress string             `json:"contractSyncerAddress"`
	LeaderElection        LeaderElectionSpec `json:"leaderElection,omitempty"`
}

type APIServerServerSpec struct {
	HTTPPort string `json:"httpPort,omitempty"`
	GRPCPort string `json:"grpcPort,omitempty"`
	Host     string `json:"host,omitempty"`
}

type APIServerStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

// GatewayController runs xDS, gRPC to API Server, and optional Kubernetes discovery.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=gatewaycontrollers,shortName=merionyxgc
type GatewayController struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewayControllerSpec   `json:"spec,omitempty"`
	Status GatewayControllerStatus `json:"status,omitempty"`
}

type GatewayControllerSpec struct {
	PodWorkloadSpec `json:",inline"`

	Server           GatewayControllerServerSpec `json:"server"`
	Etcd             EtcdSpec                    `json:"etcd"`
	APIServerAddress string                      `json:"apiServerAddress"`
	Tenant           string                      `json:"tenant,omitempty"`
	HA               GatewayControllerHASpec     `json:"ha,omitempty"`
	LeaderElection   LeaderElectionSpec          `json:"leaderElection,omitempty"`

	KubernetesDiscovery *KubernetesDiscoverySpec `json:"kubernetesDiscovery,omitempty"`
	GlobalUpstreams     []GlobalUpstreamSpec     `json:"globalUpstreams,omitempty"`
	// ExtraConfigFiles mounts additional raw YAML keys into the generated controller config (advanced).
	ExtraControllerConfig string `json:"extraControllerConfig,omitempty"`
}

type GatewayControllerServerSpec struct {
	HTTP1Port string `json:"http1Port,omitempty"`
	HTTP2Port string `json:"http2Port,omitempty"`
	GRPCPort  string `json:"grpcPort,omitempty"`
	XDSPort   string `json:"xdsPort,omitempty"`
	Host      string `json:"host,omitempty"`
}

type GatewayControllerHASpec struct {
	ControllerID string `json:"controllerId,omitempty"`
}

type GatewayControllerStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

// ContractSyncer pulls contract repositories and syncs to API Server via gRPC.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=contractsyncers,shortName=merionyxcs
type ContractSyncer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContractSyncerSpec   `json:"spec,omitempty"`
	Status ContractSyncerStatus `json:"status,omitempty"`
}

type ContractSyncerSpec struct {
	PodWorkloadSpec `json:",inline"`

	Server           ContractSyncerServerSpec `json:"server"`
	Etcd             EtcdSpec                 `json:"etcd"`
	APIServerAddress string                   `json:"apiServerAddress"`

	// RepositoryLabelSelector selects ContractRepository objects cluster-wide (namespace labels may still apply).
	RepositoryLabelSelector map[string]string `json:"repositoryLabelSelector,omitempty"`
	// ConfigMapName for aggregated repositories YAML (created by operator). Default: metadata.name + "-repositories".
	ConfigMapName string `json:"configMapName,omitempty"`
}

type ContractSyncerServerSpec struct {
	GRPCPort string `json:"grpcPort,omitempty"`
	Host     string `json:"host,omitempty"`
}

type ContractSyncerStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	RepositoriesHash   string             `json:"repositoriesHash,omitempty"`
}

// EnvoyGateway is a data-plane pod (Envoy + auth sidecar) for one logical environment.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=envoygateways,shortName=merionyxeg
type EnvoyGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvoyGatewaySpec   `json:"spec,omitempty"`
	Status EnvoyGatewayStatus `json:"status,omitempty"`
}

type EnvoyGatewaySpec struct {
	Replicas *int32 `json:"replicas,omitempty"`

	EnvoyImage       string                        `json:"envoyImage"`
	AuthSidecarImage string                        `json:"authSidecarImage"`
	ImagePullPolicy  corev1.PullPolicy             `json:"imagePullPolicy,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	Resources        corev1.ResourceRequirements   `json:"resources,omitempty"`

	// ControllerGRPCAddress is host:port for Envoy xDS (controller xds_port).
	ControllerGRPCAddress string `json:"controllerGrpcAddress"`
	// AuthControllerGRPCAddress is host:port for auth sidecar -> controller gRPC (controller grpc_port). Defaults to same host as ControllerGRPCAddress with port 19090.
	AuthControllerGRPCAddress string `json:"authControllerGrpcAddress,omitempty"`
	// Environment is the canonical environment id (must match Environment logical name / discovery).
	Environment string `json:"environment"`

	// EnvoyConfig is raw envoy bootstrap YAML. If empty, operator uses a minimal default pointing at controller xDS.
	EnvoyConfig string `json:"envoyConfig,omitempty"`
	// AuthSidecarJWKSURL is passed to auth sidecar (e.g. http://api-server-svc:8080/.well-known/jwks.json).
	AuthSidecarJWKSURL string `json:"authSidecarJwksUrl,omitempty"`
}

type EnvoyGatewayStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
type APIServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []APIServer `json:"items"`
}

// +kubebuilder:object:root=true
type GatewayControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GatewayController `json:"items"`
}

// +kubebuilder:object:root=true
type ContractSyncerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContractSyncer `json:"items"`
}

// +kubebuilder:object:root=true
type EnvoyGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EnvoyGateway `json:"items"`
}
