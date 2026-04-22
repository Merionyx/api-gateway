package models

// Provenance is the winning static config layer (ADR 0001). Extend with new fields as needed.
type Provenance struct {
	ConfigSource string `json:"config_source,omitempty"`
}

// EnvironmentMeta groups observability for a logical environment (separate from name/bundles/services).
type EnvironmentMeta struct {
	Provenance            *Provenance `json:"provenance,omitempty"`
	EffectiveGeneration   *int64      `json:"effective_generation,omitempty"`
	SourcesFingerprint    string      `json:"sources_fingerprint,omitempty"`
}

// BundleMeta is control-plane metadata for a bundle line.
type BundleMeta struct {
	Provenance *Provenance `json:"provenance,omitempty"`
}

// ServiceMeta is control-plane metadata for a static service line.
type ServiceMeta struct {
	Provenance *Provenance `json:"provenance,omitempty"`
}

type ControllerInfo struct {
	ControllerID string
	Tenant       string
	Environments []EnvironmentInfo
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
	Meta     *ServiceMeta `json:"meta,omitempty"`
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
