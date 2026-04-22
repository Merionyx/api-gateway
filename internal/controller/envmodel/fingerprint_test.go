package envmodel

import (
	"testing"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

func TestFingerprintStaticEnvironment_stable(t *testing.T) {
	a := &models.Environment{
		Name: "n", Type: "t",
		Bundles: &models.EnvironmentBundleConfig{Static: []models.StaticContractBundleConfig{
			{Name: "b2", Repository: "r2", Ref: "2", Path: "p2"},
			{Name: "b1", Repository: "r1", Ref: "1", Path: "p1"},
		}},
		Services: &models.EnvironmentServiceConfig{Static: []models.StaticServiceConfig{
			{Name: "s2", Upstream: "u2"},
			{Name: "s1", Upstream: "u1"},
		}},
	}
	b := &models.Environment{
		Name: "n", Type: "t",
		Bundles: &models.EnvironmentBundleConfig{Static: []models.StaticContractBundleConfig{
			{Name: "b1", Repository: "r1", Ref: "1", Path: "p1"},
			{Name: "b2", Repository: "r2", Ref: "2", Path: "p2"},
		}},
		Services: &models.EnvironmentServiceConfig{Static: []models.StaticServiceConfig{
			{Name: "s1", Upstream: "u1"},
			{Name: "s2", Upstream: "u2"},
		}},
	}
	if FingerprintStaticEnvironment(a) != FingerprintStaticEnvironment(b) {
		t.Fatal("order should not change fingerprint")
	}
}
