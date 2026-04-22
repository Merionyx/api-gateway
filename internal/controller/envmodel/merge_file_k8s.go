package envmodel

import "github.com/merionyx/api-gateway/internal/controller/domain/models"

// MergeFileAndK8s overlays Kubernetes-discovered bundles/services onto static (file) config
// for the same logical environment. If both define the same bundle key (see BundleKey) or
// the same service name, the Kubernetes entry wins. Name/Type: Kubernetes when non-nil, else file.
// Snapshots are cleared in the result.
func MergeFileAndK8s(file, k8s *models.Environment) *models.Environment {
	var fBundles, kBundles []models.StaticContractBundleConfig
	if file != nil && file.Bundles != nil {
		fBundles = file.Bundles.Static
	}
	if k8s != nil && k8s.Bundles != nil {
		kBundles = k8s.Bundles.Static
	}
	var fSvc, kSvc []models.StaticServiceConfig
	if file != nil && file.Services != nil {
		fSvc = file.Services.Static
	}
	if k8s != nil && k8s.Services != nil {
		kSvc = k8s.Services.Static
	}
	name := ""
	typ := "kubernetes"
	if k8s != nil {
		name = k8s.Name
		typ = k8s.Type
	} else if file != nil {
		name = file.Name
		typ = file.Type
	}
	return &models.Environment{
		Name: name,
		Type: typ,
		Bundles: &models.EnvironmentBundleConfig{
			Static: UnionStaticBundles(fBundles, kBundles),
		},
		Services: &models.EnvironmentServiceConfig{
			Static: UnionStaticServices(fSvc, kSvc),
		},
		Snapshots: nil,
	}
}
