package envmodel

import "github.com/merionyx/api-gateway/internal/controller/domain/models"

// ToAPIServerSkeleton returns a copy of the environment for API server sync: static bundles
// and services are shallow-copied, snapshots cleared. Nil input yields nil.
func ToAPIServerSkeleton(e *models.Environment) *models.Environment {
	if e == nil {
		return nil
	}
	return &models.Environment{
		Name:      e.Name,
		Type:      e.Type,
		Bundles:   cloneEnvBundles(e.Bundles),
		Services:  cloneEnvServices(e.Services),
		Snapshots: nil,
	}
}

func cloneEnvBundles(b *models.EnvironmentBundleConfig) *models.EnvironmentBundleConfig {
	if b == nil {
		return &models.EnvironmentBundleConfig{Static: nil}
	}
	cp := make([]models.StaticContractBundleConfig, len(b.Static))
	copy(cp, b.Static)
	return &models.EnvironmentBundleConfig{Static: cp}
}

func cloneEnvServices(s *models.EnvironmentServiceConfig) *models.EnvironmentServiceConfig {
	if s == nil {
		return &models.EnvironmentServiceConfig{Static: nil}
	}
	cp := make([]models.StaticServiceConfig, len(s.Static))
	copy(cp, s.Static)
	return &models.EnvironmentServiceConfig{Static: cp}
}
