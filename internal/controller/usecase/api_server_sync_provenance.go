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

func provenancePB(src envmodel.StaticConfigSource) *pb.Provenance {
	v := staticConfigToPB(src)
	if v == pb.ConfigSource_CONFIG_SOURCE_UNSPECIFIED {
		return nil
	}
	return &pb.Provenance{ConfigSource: v}
}

// fileK8sSlices returns unmerged file and K8s static bundles if the in-memory implementation supports it.
func (b *registryEnvironmentsBuilder) fileK8sSlices(ctx context.Context, envName string) (file, k8s []models.StaticContractBundleConfig) {
	if b.inMemoryEnvironmentsRepo == nil {
		return nil, nil
	}
	// direct method on *memory.EnvironmentsRepository
	if m, ok := b.inMemoryEnvironmentsRepo.(*memory.EnvironmentsRepository); ok {
		return m.FileAndK8sStaticBundles(ctx, envName)
	}
	if p, ok := b.inMemoryEnvironmentsRepo.(fileK8sPartials); ok {
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

func etcdStaticServices(etcd *models.Environment) []models.StaticServiceConfig {
	if etcd == nil || etcd.Services == nil {
		return nil
	}
	return etcd.Services.Static
}

type fileK8sServicePartials interface {
	FileAndK8sStaticServices(ctx context.Context, environmentName string) (file, k8s []models.StaticServiceConfig)
}

// fileK8sServiceSlices returns unmerged file and K8s static service lists.
func (b *registryEnvironmentsBuilder) fileK8sServiceSlices(ctx context.Context, envName string) (file, k8s []models.StaticServiceConfig) {
	if b.inMemoryEnvironmentsRepo == nil {
		return nil, nil
	}
	if m, ok := b.inMemoryEnvironmentsRepo.(*memory.EnvironmentsRepository); ok {
		return m.FileAndK8sStaticServices(ctx, envName)
	}
	if p, ok := b.inMemoryEnvironmentsRepo.(fileK8sServicePartials); ok {
		return p.FileAndK8sStaticServices(ctx, envName)
	}
	return nil, nil
}

type environmentLayersPartials interface {
	EnvironmentLayersPresent(environmentName string) (inFile, inK8s bool)
}

// environmentInMemoryLayers returns (inFile, inK8s) for environment name, or (false, false) if not supported.
func (b *registryEnvironmentsBuilder) environmentInMemoryLayers(envName string) (inFile, inK8s bool) {
	if b.inMemoryEnvironmentsRepo == nil {
		return false, false
	}
	if p, ok := b.inMemoryEnvironmentsRepo.(environmentLayersPartials); ok {
		return p.EnvironmentLayersPresent(envName)
	}
	return false, false
}
