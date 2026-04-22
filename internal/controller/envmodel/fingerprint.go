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
		p.Bundles = make([]models.StaticContractBundleConfig, len(e.Bundles.Static))
		copy(p.Bundles, e.Bundles.Static)
	}
	if e.Services != nil {
		p.Services = make([]models.StaticServiceConfig, len(e.Services.Static))
		copy(p.Services, e.Services.Static)
	}
	sort.Slice(p.Bundles, func(i, j int) bool { return BundleKey(p.Bundles[i]) < BundleKey(p.Bundles[j]) })
	sort.Slice(p.Services, func(i, j int) bool { return p.Services[i].Name < p.Services[j].Name })
	b, _ := json.Marshal(p)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type staticFingerprintPayload struct {
	Name     string                               `json:"name"`
	Type     string                               `json:"type"`
	Bundles  []models.StaticContractBundleConfig  `json:"bundles"`
	Services []models.StaticServiceConfig         `json:"services"`
}
