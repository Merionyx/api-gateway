package interfaces

import (
	"context"

	"github.com/merionyx/api-gateway/internal/api-server/domain/models"
	sharedgit "github.com/merionyx/api-gateway/internal/shared/git"
)

// ContractSyncRemote pulls contract snapshots from Contract Syncer (gRPC Sync RPC). Does not write etcd.
type ContractSyncRemote interface {
	FetchContractSnapshots(ctx context.Context, bundle models.BundleInfo) ([]sharedgit.ContractSnapshot, error)
}

// ContractExportRemote exports contract files via Contract Syncer (gRPC ExportContracts RPC).
type ContractExportRemote interface {
	ExportContractFiles(ctx context.Context, repository, ref, path, contractName string) ([]sharedgit.ExportedContractFile, error)
}

// ContractSyncerReachability verifies that the Contract Syncer gRPC endpoint accepts connections (e.g. for /status).
type ContractSyncerReachability interface {
	Ping(ctx context.Context) error
}
