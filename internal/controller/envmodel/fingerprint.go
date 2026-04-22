package envmodel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

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
