package usecase

import (
	"context"
	"time"

	"github.com/merionyx/api-gateway/internal/shared/grpcobs"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// StatusReadUseCase aggregates dependency health for GET /api/v1/status.
type StatusReadUseCase struct {
	etcdClient         *clientv3.Client
	contractSyncerAddr string
	contractSyncerTLS  grpcobs.ClientTLSConfig
}

func NewStatusReadUseCase(
	etcdClient *clientv3.Client,
	contractSyncerAddr string,
	contractSyncerTLS grpcobs.ClientTLSConfig,
) *StatusReadUseCase {
	return &StatusReadUseCase{
		etcdClient:         etcdClient,
		contractSyncerAddr: contractSyncerAddr,
		contractSyncerTLS:  contractSyncerTLS,
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
	if u.contractSyncerAddr == "" {
		return "error"
	}
	opts, err := ContractSyncerDialOptions(u.contractSyncerTLS)
	if err != nil {
		return "error"
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(u.contractSyncerAddr, opts...)
	if err != nil {
		return "error"
	}
	defer func() { _ = conn.Close() }()
	conn.Connect()
	for {
		st := conn.GetState()
		if st == connectivity.Ready {
			return "ok"
		}
		if st == connectivity.Shutdown {
			return "error"
		}
		if !conn.WaitForStateChange(ctx, st) {
			return "error"
		}
	}
}
