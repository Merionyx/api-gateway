package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"merionyx/api-gateway/internal/contract-syncer/container"
	"merionyx/api-gateway/internal/shared/grpcobs"
	"merionyx/api-gateway/internal/shared/serviceapp"
	pb "merionyx/api-gateway/pkg/api/contract_syncer/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// RunGRPCServer serves the sync API until ctx is cancelled.
func RunGRPCServer(ctx context.Context, cnt *container.Container) error {
	address := fmt.Sprintf("%s:%s", cnt.Config.Server.Host, cnt.Config.Server.GRPCPort)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	opts, err := grpcobs.ServerOptions(&cnt.Config.Server.GRPC.TLS, cnt.Config.Server.GRPC.Observability, cnt.Config.MetricsHTTP.Enabled)
	if err != nil {
		return fmt.Errorf("gRPC server options: %w", err)
	}
	grpcSrv := grpc.NewServer(opts...)

	pb.RegisterContractSyncerServiceServer(grpcSrv, cnt.SyncGRPCHandler)

	if cnt.Config.Server.GRPC.Observability.ReflectionEnabled {
		reflection.Register(grpcSrv)
	}

	slog.Info("Starting gRPC server", "address", address)
	return serviceapp.RunGRPCServeUntil(ctx, grpcSrv, lis)
}
