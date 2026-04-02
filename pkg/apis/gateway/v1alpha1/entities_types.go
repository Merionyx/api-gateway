package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Environment declares a logical environment; bundles and services are discovered separately.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=environments,shortName=merionyxenv
type Environment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvironmentSpec   `json:"spec,omitempty"`
	Status EnvironmentStatus `json:"status,omitempty"`
}

type EnvironmentSpec struct {
	// LogicalName is the canonical id for API Server / xDS / EnvoyGateway.environment.
	// If empty, defaults to "<metadata.namespace>-<metadata.name>".
	LogicalName string `json:"logicalName,omitempty"`
}

type EnvironmentStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ResolvedLogicalName string            `json:"resolvedLogicalName,omitempty"`
}

// ContractBundle binds a repository ref to an environment.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=contractbundles,shortName=merionyxbun
type ContractBundle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContractBundleSpec   `json:"spec,omitempty"`
	Status ContractBundleStatus `json:"status,omitempty"`
}

type ContractBundleSpec struct {
	// Display name for the bundle (OpenAPI grouping etc.).
	Name string `json:"name"`
	// Repository is the logical name of a ContractRepository CR (metadata.name).
	Repository string `json:"repository"`
	Ref        string `json:"ref"`
	Path       string `json:"path"`

	// EnvironmentRef points to an Environment CR.
	EnvironmentRef *corev1.ObjectReference `json:"environmentRef,omitempty"`
	// EnvironmentID is an alternative to EnvironmentRef (label gateway.merionyx.io/environment-id may also be used).
	EnvironmentID string `json:"environmentId,omitempty"`
}

type ContractBundleStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

// ContractRepository describes a git or local contract source for Contract Syncer.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=contractrepositories,shortName=merionyxrepo
type ContractRepository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContractRepositorySpec   `json:"spec,omitempty"`
	Status ContractRepositoryStatus `json:"status,omitempty"`
}

type ContractRepositorySpec struct {
	// Source is one of: git, local-git, local-dir
	Source string `json:"source"`
	URL    string `json:"url,omitempty"`
	Path   string `json:"path,omitempty"`
	Auth   ContractRepositoryAuth   `json:"auth,omitempty"`
}

type ContractRepositoryAuth struct {
	Type string `json:"type,omitempty"`
	// SecretRef provides keys for token or SSH (consumed when generating syncer config).
	SecretRef *corev1.LocalObjectReference `json:"secretRef,omitempty"`
	// TokenEnv and SSHKeyEnv are environment variable names set on Contract Syncer pod from Secret keys.
	TokenEnv   string `json:"tokenEnv,omitempty"`
	SSHKeyEnv  string `json:"sshKeyEnv,omitempty"`
	SSHKeyPath string `json:"sshKeyPath,omitempty"`
}

type ContractRepositoryStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

// GatewayUpstream is an explicit backend URL; omit EnvironmentID for global upstreams.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=gatewayupstreams,shortName=merionyxup
type GatewayUpstream struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewayUpstreamSpec   `json:"spec,omitempty"`
	Status GatewayUpstreamStatus `json:"status,omitempty"`
}

type GatewayUpstreamSpec struct {
	Name     string `json:"name"`
	Upstream string `json:"upstream"`
	// EnvironmentID matches Environment logical name; empty means global (all environments see it).
	EnvironmentID string `json:"environmentId,omitempty"`
}

type GatewayUpstreamStatus struct {
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
type EnvironmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Environment `json:"items"`
}

// +kubebuilder:object:root=true
type ContractBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContractBundle `json:"items"`
}

// +kubebuilder:object:root=true
type ContractRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContractRepository `json:"items"`
}

// +kubebuilder:object:root=true
type GatewayUpstreamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GatewayUpstream `json:"items"`
}
