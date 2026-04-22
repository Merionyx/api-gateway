package envmodel

import "github.com/merionyx/api-gateway/internal/controller/domain/models"

// StaticBundleSlice returns the static bundle slice, or nil if the environment or bundles are nil.
func StaticBundleSlice(e *models.Environment) []models.StaticContractBundleConfig {
	if e == nil || e.Bundles == nil {
		return nil
	}
	return e.Bundles.Static
}

// StaticServiceSlice returns the static service slice, or nil if the environment or services are nil.
func StaticServiceSlice(e *models.Environment) []models.StaticServiceConfig {
	if e == nil || e.Services == nil {
		return nil
	}
	return e.Services.Static
}
