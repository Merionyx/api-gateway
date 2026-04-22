package models

type ControllerInfo struct {
	ControllerID string
	Tenant       string
	Environments []EnvironmentInfo
}

type EnvironmentInfo struct {
	Name    string
	Bundles []BundleInfo
	// EffectiveGeneration is the materialized effective document generation when reported by the controller.
	EffectiveGeneration *int64 `json:"effective_generation,omitempty"`
	// SourcesFingerprint is a hex hash of static name/type/bundles/services (aligned with controller materialized).
	SourcesFingerprint string `json:"sources_fingerprint,omitempty"`
	// EnvironmentConfigSource is the dominant layer for the environment name: etcd > kubernetes > file.
	EnvironmentConfigSource string `json:"environment_config_source,omitempty"`
	// Services are effective static upstreams with per-line provenance.
	Services []ServiceInfo `json:"services,omitempty"`
}

// ServiceInfo is a static service line with config provenance.
type ServiceInfo struct {
	Name     string
	Upstream string
	// ConfigSource is the winning input for this service name in the effective merge.
	ConfigSource string `json:"config_source,omitempty"`
}

type BundleInfo struct {
	Name       string
	Repository string
	Ref        string
	Path       string
	// ConfigSource is the winning input for this bundle key in the effective merge: file | kubernetes | etcd_grpc.
	ConfigSource string `json:"config_source,omitempty"`
}

