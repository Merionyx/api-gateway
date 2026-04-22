package envmodel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

// FingerprintK8sDiscovery is a single stable hash of the K8s discovery result: per-name
// environments (static bundles and services) plus cluster-global upstreams. Ordering of map
// iteration and list appends is normalized; [DiscoveryRef] is excluded, matching
// [FingerprintStaticEnvironment].
func FingerprintK8sDiscovery(envs map[string]*models.Environment, globals []models.StaticServiceConfig) string {
	keys := make([]string, 0, len(envs))
	for k := range envs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	entries := make([]k8sDiscoveryEnvEntry, 0, len(keys))
	for _, name := range keys {
		entries = append(entries, k8sDiscoveryEnvEntry{
			Name:          name,
			StaticStateFP: FingerprintStaticEnvironment(envs[name]),
		})
	}
	gs := make([]k8sDiscoveryServiceRow, 0, len(globals))
	for i := range globals {
		s := &globals[i]
		gs = append(gs, k8sDiscoveryServiceRow{Name: s.Name, Upstream: s.Upstream})
	}
	sort.Slice(gs, func(i, j int) bool {
		if gs[i].Name != gs[j].Name {
			return gs[i].Name < gs[j].Name
		}
		return gs[i].Upstream < gs[j].Upstream
	})
	p := k8sDiscoveryFPPayload{Environments: entries, GlobalServices: gs}
	b, _ := json.Marshal(p)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type k8sDiscoveryEnvEntry struct {
	Name          string `json:"name"`
	StaticStateFP string `json:"staticStateFp"`
}

type k8sDiscoveryServiceRow struct {
	Name     string `json:"name"`
	Upstream string `json:"upstream"`
}

type k8sDiscoveryFPPayload struct {
	Environments   []k8sDiscoveryEnvEntry   `json:"environments"`
	GlobalServices []k8sDiscoveryServiceRow `json:"globalServices"`
}

// FingerprintStaticEnvironment returns a stable hex-encoded SHA-256 of the name, type, and
// static bundles and services, for idempotent materialized effective writes. Snapshots are
// not included.
func FingerprintStaticEnvironment(e *models.Environment) string {
	if e == nil {
		return "nil"
	}
	p := staticFingerprintPayload{
		Name: e.Name,
		Type: e.Type,
	}
	if e.Bundles != nil {
		p.Bundles = make([]bundleForFingerprint, len(e.Bundles.Static))
		for i := range e.Bundles.Static {
			b := e.Bundles.Static[i]
			p.Bundles[i] = bundleForFingerprint{Name: b.Name, Repository: b.Repository, Ref: b.Ref, Path: b.Path}
		}
	}
	if e.Services != nil {
		p.Services = make([]serviceForFingerprint, len(e.Services.Static))
		for i := range e.Services.Static {
			s := e.Services.Static[i]
			p.Services[i] = serviceForFingerprint{Name: s.Name, Upstream: s.Upstream}
		}
	}
	sort.Slice(p.Bundles, func(i, j int) bool { return bundleKeyForFP(p.Bundles[i]) < bundleKeyForFP(p.Bundles[j]) })
	sort.Slice(p.Services, func(i, j int) bool { return p.Services[i].Name < p.Services[j].Name })
	b, _ := json.Marshal(p)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type bundleForFingerprint struct {
	Name       string `json:"name"`
	Repository string `json:"repository"`
	Ref        string `json:"ref"`
	Path       string `json:"path"`
}

type serviceForFingerprint struct {
	Name     string `json:"name"`
	Upstream string `json:"upstream"`
}

func bundleKeyForFP(b bundleForFingerprint) string {
	return BundleKey(models.StaticContractBundleConfig{
		Name: b.Name, Repository: b.Repository, Ref: b.Ref, Path: b.Path,
	})
}

type staticFingerprintPayload struct {
	Name     string               `json:"name"`
	Type     string               `json:"type"`
	Bundles  []bundleForFingerprint  `json:"bundles"`
	Services []serviceForFingerprint `json:"services"`
}
