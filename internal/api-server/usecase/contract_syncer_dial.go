package usecase

import (
	"time"

	"merionyx/api-gateway/internal/shared/grpcobs"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// ContractSyncerDialOptions returns gRPC dial options for the Contract Syncer client (TLS + keepalive).
func ContractSyncerDialOptions(tls grpcobs.ClientTLSConfig) ([]grpc.DialOption, error) {
	tlsOpts, err := grpcobs.DialOptions(tls)
	if err != nil {
		return nil, err
	}
	return append(tlsOpts,
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                20 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	), nil
}
