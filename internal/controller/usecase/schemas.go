package usecase

import (
	"context"
	"errors"
	"fmt"

	"merionyx/api-gateway/internal/controller/domain/interfaces"
	"merionyx/api-gateway/internal/controller/domain/models"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// ErrContractsManagedByAPIServer is returned for SyncContractBundle / SyncAllContracts:
// contract data is pushed from API Server; the controller does not pull from Git.
var ErrContractsManagedByAPIServer = errors.New("contracts are delivered by API Server; controller does not sync from Git")

type schemasUseCase struct {
	schemaRepo      interfaces.SchemaRepository
	environmentRepo interfaces.EnvironmentRepository
}

func NewSchemasUseCase() interfaces.SchemasUseCase {
	return &schemasUseCase{}
}

func (uc *schemasUseCase) SetDependencies(schemaRepo interfaces.SchemaRepository, environmentRepo interfaces.EnvironmentRepository) {
	uc.schemaRepo = schemaRepo
	uc.environmentRepo = environmentRepo
}

func (uc *schemasUseCase) SyncContractBundle(ctx context.Context, req *models.SyncContractBundleRequest) (*models.SyncContractBundleResponse, error) {
	_ = ctx
	_ = req
	return nil, fmt.Errorf("%w", ErrContractsManagedByAPIServer)
}

func (uc *schemasUseCase) GetContractSnapshot(ctx context.Context, repository, ref, contract string) (*models.ContractSnapshot, error) {
	return uc.schemaRepo.GetContractSnapshot(ctx, repository, ref, contract)
}

func (uc *schemasUseCase) ListContractSnapshots(ctx context.Context, repository, ref string) ([]models.ContractSnapshot, error) {
	return uc.schemaRepo.ListContractSnapshots(ctx, repository, ref)
}

func (uc *schemasUseCase) SyncAllContracts(ctx context.Context, req *models.SyncAllContractsRequest) (*models.SyncAllContractsResponse, error) {
	_ = ctx
	_ = req
	return nil, fmt.Errorf("%w", ErrContractsManagedByAPIServer)
}

func (uc *schemasUseCase) WatchContractBundlesSnapshots(ctx context.Context) clientv3.WatchChan {
	return uc.schemaRepo.WatchContractBundlesSnapshots(ctx)
}
