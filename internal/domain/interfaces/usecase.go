package interfaces

import (
	"context"

	"merionyx/api-gateway/control-plane/internal/domain/models"
	"merionyx/api-gateway/control-plane/internal/repository/git"
)

// SnapshotsUseCase interface for xDS snapshots business logic
type SnapshotsUseCase interface {
	UpdateSnapshot(ctx context.Context, req *models.UpdateSnapshotRequest) (*models.UpdateSnapshotResponse, error)
	GetSnapshotStatus(ctx context.Context, req *models.GetSnapshotStatusRequest) (*models.GetSnapshotStatusResponse, error)
}

// EnvironmentsUseCase interface for environments business logic
type EnvironmentsUseCase interface {
	CreateEnvironment(ctx context.Context, req *models.CreateEnvironmentRequest) (*models.Environment, error)
	GetEnvironment(ctx context.Context, name string) (*models.Environment, error)
	ListEnvironments(ctx context.Context) (map[string]*models.Environment, error)
	UpdateEnvironment(ctx context.Context, req *models.UpdateEnvironmentRequest) (*models.Environment, error)
	DeleteEnvironment(ctx context.Context, name string) error
}

// SchemasUseCase interface for schemas/contracts business logic
type SchemasUseCase interface {
	SyncContractBundle(ctx context.Context, req *models.SyncContractBundleRequest) (*models.SyncContractBundleResponse, error)
	GetContractSnapshot(ctx context.Context, repository, ref, contract string) (*git.ContractSnapshot, error)
	ListContractSnapshots(ctx context.Context, repository, ref string) ([]git.ContractSnapshot, error)
	SyncAllContracts(ctx context.Context, req *models.SyncAllContractsRequest) (*models.SyncAllContractsResponse, error)
}
