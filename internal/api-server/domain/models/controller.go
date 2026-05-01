package models

// Provenance is the winning static config layer (ADR 0001). Extend with new fields as needed.
type Provenance struct {
	ConfigSource string `json:"config_source,omitempty"`
	LayerDetail  string `json:"layer_detail,omitempty"`
}

// EnvironmentMeta groups observability for a logical environment (separate from name/bundles/services).
type EnvironmentMeta struct {
	Provenance                *Provenance `json:"provenance,omitempty"`
	EffectiveGeneration       *int64      `json:"effective_generation,omitempty"`
	SourcesFingerprint        string      `json:"sources_fingerprint,omitempty"`
	EnvironmentType           string      `json:"environment_type,omitempty"`
	MaterializedUpdatedAt     string      `json:"materialized_updated_at,omitempty"`
	MaterializedSchemaVersion *int32      `json:"materialized_schema_version,omitempty"`
	MaterializedMismatch      *bool       `json:"materialized_mismatch,omitempty"`
}

// BundleMeta is control-plane metadata for a bundle line.
type BundleMeta struct {
	Provenance     *Provenance `json:"provenance,omitempty"`
	ResolvedRef    string      `json:"resolved_ref,omitempty"`
	LastSyncUTC    string      `json:"last_sync_utc,omitempty"`
	SyncError      string      `json:"sync_error,omitempty"`
	K8SResourceRef string      `json:"k8s_resource_ref,omitempty"`
}

// ServiceMeta is control-plane metadata for a static service line.
type ServiceMeta struct {
	Provenance    *Provenance `json:"provenance,omitempty"`
	K8sServiceRef string      `json:"k8s_service_ref,omitempty"`
}

type ControllerInfo struct {
	ControllerID           string
	Tenant                 string
	Environments           []EnvironmentInfo
	RegistryPayloadVersion int32 `json:"registry_payload_version,omitempty"`
}

type EnvironmentInfo struct {
	Name     string
	Bundles  []BundleInfo
	Services []ServiceInfo
	Meta     *EnvironmentMeta `json:"meta,omitempty"`
}

// ServiceInfo is a static service line; meta holds provenance and future fields.
type ServiceInfo struct {
	Name     string
	Upstream string
	// Scope: "environment" | "controller_root" from controller.
	Scope string       `json:"scope,omitempty"`
	Meta  *ServiceMeta `json:"meta,omitempty"`
}

type BundleInfo struct {
	Name       string
	Repository string
	Ref        string
	Path       string
	Meta       *BundleMeta `json:"meta,omitempty"`
}

// EnvironmentMetaConfigSource returns the dominant config_source string for merge rules, or "".
func EnvironmentMetaConfigSource(m *EnvironmentMeta) string {
	if m == nil || m.Provenance == nil {
		return ""
	}
	return m.Provenance.ConfigSource
}
