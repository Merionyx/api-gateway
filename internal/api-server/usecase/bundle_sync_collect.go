package usecase

import (
	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	"github.com/merionyx/api-gateway/internal/shared/bundlekey"
)

// collectUniqueBundles merges bundles from all registered controllers keyed by bundlekey.Build.
func collectUniqueBundles(controllers []models.ControllerInfo) []models.BundleInfo {
	bundlesMap := make(map[string]models.BundleInfo)
	for _, controller := range controllers {
		for _, env := range controller.Environments {
			for _, bundle := range env.Bundles {
				key := bundlekey.Build(bundle.Repository, bundle.Ref, bundle.Path)
				bundlesMap[key] = bundle
			}
		}
	}
	out := make([]models.BundleInfo, 0, len(bundlesMap))
	for _, b := range bundlesMap {
		out = append(out, b)
	}
	return out
}
