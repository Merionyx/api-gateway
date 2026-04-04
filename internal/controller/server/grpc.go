package server

import (
	"fmt"
	"log/slog"
	"merionyx/api-gateway/internal/controller/container"
	"merionyx/api-gateway/internal/shared/grpcobs"
	authv1 "merionyx/api-gateway/pkg/api/auth/v1"
	environmentsv1 "merionyx/api-gateway/pkg/api/environments/v1"
	schemasv1 "merionyx/api-gateway/pkg/api/schemas/v1"
	snapshotsv1 "merionyx/api-gateway/pkg/api/snapshots/v1"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func StartGRPCServer(container *container.Container) error {
	lis, err := net.Listen("tcp", ":"+container.Config.Server.GRPCPort)
	if err != nil {
		return fmt.Errorf("failed to listen on :%s: %w", container.Config.Server.GRPCPort, err)
	}

	opts, err := grpcobs.ServerOptions(&container.Config.GRPCControlPlane.TLS, container.Config.GRPCControlPlane.Observability)
	if err != nil {
		return fmt.Errorf("control-plane gRPC options: %w", err)
	}
	server := grpc.NewServer(opts...)

	// Register services
	snapshotsv1.RegisterSnapshotsServiceServer(server, container.SnapshotGRPCHandler)
	environmentsv1.RegisterEnvironmentsServiceServer(server, container.EnvironmentsGRPCHandler)
	schemasv1.RegisterSchemasServiceServer(server, container.SchemasGRPCHandler)
	authv1.RegisterAuthServiceServer(server, container.AuthGRPCHandler)

	if container.Config.GRPCControlPlane.Observability.ReflectionEnabled {
		reflection.Register(server)
	}

	slog.Info("gRPC server starting", "port", container.Config.Server.GRPCPort)
	return server.Serve(lis)
}
