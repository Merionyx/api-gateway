package envmodel

import "github.com/merionyx/api-gateway/internal/controller/domain/models"

// ConfigSourceForEnvironmentLayers reports the dominant static-config layer that declares the
// environment name (ADR: etcd over in-memory, then in-memory: K8s over file).
func ConfigSourceForEnvironmentLayers(inFile, inK8s, inEtcd bool) StaticConfigSource {
	if inEtcd {
		return StaticConfigEtcdGRPC
	}
	if inK8s {
		return StaticConfigKubernetes
	}
	if inFile {
		return StaticConfigFile
	}
	return StaticConfigUnspecified
}

// ConfigSourceForStaticService reports the winning source for a service by name in the
// effective list (order: etcd > k8s > file). Slices are unmerged lists from the three layers.
func ConfigSourceForStaticService(
	eff models.StaticServiceConfig,
	file, k8s, etcd []models.StaticServiceConfig,
) StaticConfigSource {
	n := eff.Name
	if inServiceNameSet(etcd, n) {
		return StaticConfigEtcdGRPC
	}
	if inServiceNameSet(k8s, n) {
		return StaticConfigKubernetes
	}
	if inServiceNameSet(file, n) {
		return StaticConfigFile
	}
	return StaticConfigUnspecified
}

func inServiceNameSet(s []models.StaticServiceConfig, name string) bool {
	if len(s) == 0 {
		return false
	}
	for i := range s {
		if s[i].Name == name {
			return true
		}
	}
	return false
}
