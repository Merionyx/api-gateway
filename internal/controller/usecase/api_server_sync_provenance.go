package usecase

import (
	"context"

	pb "github.com/merionyx/api-gateway/pkg/grpc/controller_registry/v1"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
	"github.com/merionyx/api-gateway/internal/controller/envmodel"
	"github.com/merionyx/api-gateway/internal/controller/repository/memory"
)

type fileK8sPartials interface {
	FileAndK8sStaticBundles(ctx context.Context, environmentName string) (file, k8s []models.StaticContractBundleConfig)
}

func staticConfigToPB(s envmodel.StaticConfigSource) pb.ConfigSource {
	switch s {
	case envmodel.StaticConfigFile:
		return pb.ConfigSource_CONFIG_SOURCE_FILE
	case envmodel.StaticConfigKubernetes:
		return pb.ConfigSource_CONFIG_SOURCE_KUBERNETES
	case envmodel.StaticConfigEtcdGRPC:
		return pb.ConfigSource_CONFIG_SOURCE_ETCD_GRPC
	default:
		return pb.ConfigSource_CONFIG_SOURCE_UNSPECIFIED
	}
}

// fileK8sSlices returns unmerged file and K8s static bundles if the in-memory implementation supports it.
func (uc *APIServerSyncUseCase) fileK8sSlices(ctx context.Context, envName string) (file, k8s []models.StaticContractBundleConfig) {
	if uc.inMemoryEnvironmentsRepo == nil {
		return nil, nil
	}
	// direct method on *memory.EnvironmentsRepository
	if m, ok := uc.inMemoryEnvironmentsRepo.(*memory.EnvironmentsRepository); ok {
		return m.FileAndK8sStaticBundles(ctx, envName)
	}
	if p, ok := uc.inMemoryEnvironmentsRepo.(fileK8sPartials); ok {
		return p.FileAndK8sStaticBundles(ctx, envName)
	}
	return nil, nil
}

func etcdStaticBundles(etcd *models.Environment) []models.StaticContractBundleConfig {
	if etcd == nil || etcd.Bundles == nil {
		return nil
	}
	return etcd.Bundles.Static
}
