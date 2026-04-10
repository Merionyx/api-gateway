package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// EtcdSpec connects components to etcd (TLS client certs in a Secret: ca.crt, tls.crt, tls.key).
type EtcdSpec struct {
	Endpoints    []string                     `json:"endpoints"`
	TLSSecretRef *corev1.LocalObjectReference `json:"tlsSecretRef,omitempty"`
	DialTimeout  string                       `json:"dialTimeout,omitempty"`
}

// LeaderElectionSpec matches shared election behaviour in control-plane binaries.
type LeaderElectionSpec struct {
	Enabled           bool   `json:"enabled,omitempty"`
	KeyPrefix         string `json:"keyPrefix,omitempty"`
	SessionTTLSeconds int    `json:"sessionTTLSeconds,omitempty"`
}

// PodWorkloadSpec is common deployment configuration.
type PodWorkloadSpec struct {
	Image            string                        `json:"image"`
	ImagePullPolicy  corev1.PullPolicy             `json:"imagePullPolicy,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	Replicas         *int32                        `json:"replicas,omitempty"`
	Resources        corev1.ResourceRequirements   `json:"resources,omitempty"`
	ServiceAccount   string                        `json:"serviceAccountName,omitempty"`
	Affinity         *corev1.Affinity              `json:"affinity,omitempty"`
	// EnablePodDisruptionBudget creates a PDB when replicas > 1.
	EnablePodDisruptionBudget bool `json:"enablePodDisruptionBudget,omitempty"`
}

// KubernetesDiscoverySpec configures Gateway Controller cluster-wide watches.
type KubernetesDiscoverySpec struct {
	Enabled bool `json:"enabled"`
	// NamespaceLabelSelector filters namespaces (labels on Namespace objects).
	NamespaceLabelSelector map[string]string `json:"namespaceLabelSelector,omitempty"`
	// ResourceLabelSelector is applied to Environment, ContractBundle, GatewayUpstream list/watch.
	ResourceLabelSelector map[string]string `json:"resourceLabelSelector,omitempty"`
	// WatchNamespaces restricts to these namespaces (in addition to selectors).
	WatchNamespaces []string `json:"watchNamespaces,omitempty"`
}

// GlobalUpstreamSpec is a cluster-global backend (no environment), like controller config services.static.
type GlobalUpstreamSpec struct {
	Name     string `json:"name"`
	Upstream string `json:"upstream"`
}
