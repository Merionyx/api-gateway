package registry

import (
	"context"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/domain/interfaces"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// StatusReadUseCase aggregates dependency health for GET /api/v1/status.
type StatusReadUseCase struct {
	etcdClient         *clientv3.Client
	contractSyncerPing interfaces.ContractSyncerReachability
}

func NewStatusReadUseCase(
	etcdClient *clientv3.Client,
	contractSyncerPing interfaces.ContractSyncerReachability,
) *StatusReadUseCase {
	return &StatusReadUseCase{
		etcdClient:         etcdClient,
		contractSyncerPing: contractSyncerPing,
	}
}

// CheckEtcd performs a lightweight read against the API Server key prefix.
func (u *StatusReadUseCase) CheckEtcd(ctx context.Context) string {
	if u.etcdClient == nil {
		return "error"
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err := u.etcdClient.Get(ctx, "/api-gateway/api-server/", clientv3.WithPrefix(), clientv3.WithLimit(1))
	if err != nil {
		return "error"
	}
	return "ok"
}

// CheckContractSyncer verifies gRPC connectivity to the Contract Syncer.
func (u *StatusReadUseCase) CheckContractSyncer(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := u.contractSyncerPing.Ping(ctx); err != nil {
		return "error"
	}
	return "ok"
}
