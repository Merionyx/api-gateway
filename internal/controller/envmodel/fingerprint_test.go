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

func TestFingerprintK8sDiscovery_stable(t *testing.T) {
	e1 := &models.Environment{
		Name: "e1", Type: "kubernetes",
		Bundles:  &models.EnvironmentBundleConfig{Static: []models.StaticContractBundleConfig{{Name: "b1", Repository: "r", Ref: "1", Path: "p"}}},
		Services: &models.EnvironmentServiceConfig{Static: []models.StaticServiceConfig{{Name: "s1", Upstream: "u1", DiscoveryRef: "a/a"}}},
	}
	e2 := &models.Environment{
		Name: "e2", Type: "kubernetes",
		Bundles:  &models.EnvironmentBundleConfig{Static: []models.StaticContractBundleConfig{{Name: "b2", Repository: "r", Ref: "1", Path: "p"}}},
		Services: &models.EnvironmentServiceConfig{Static: []models.StaticServiceConfig{{Name: "s1", Upstream: "u1", DiscoveryRef: "b/b"}}},
	}
	m1 := map[string]*models.Environment{"a": e1, "b": e2}
	m2 := map[string]*models.Environment{"b": e2, "a": e1}
	g1 := []models.StaticServiceConfig{{Name: "g1", Upstream: "h1", DiscoveryRef: "x"}}
	g2 := []models.StaticServiceConfig{{Name: "g1", Upstream: "h1", DiscoveryRef: "y"}}
	if FingerprintK8sDiscovery(m1, g1) != FingerprintK8sDiscovery(m2, g1) {
		t.Fatal("map key order should not change k8s discovery fingerprint")
	}
	if FingerprintK8sDiscovery(m1, g1) != FingerprintK8sDiscovery(m1, g2) {
		t.Fatal("global DiscoveryRef should not change fingerprint")
	}
}
