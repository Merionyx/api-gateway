package server

import (
	"fmt"
	"log/slog"
	"net"

	"merionyx/api-gateway/internal/contract-syncer/container"
	pb "merionyx/api-gateway/pkg/api/contract_syncer/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func StartGRPCServer(cnt *container.Container) error {
	address := fmt.Sprintf("%s:%s", cnt.Config.Server.Host, cnt.Config.Server.GRPCPort)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer()

	pb.RegisterContractSyncerServiceServer(grpcServer, cnt.SyncGRPCHandler)

	reflection.Register(grpcServer)

	slog.Info("Starting gRPC server", "address", address)

	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}
