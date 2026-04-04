package server

import (
	"context"
	"fmt"
	"log/slog"
	"merionyx/api-gateway/internal/controller/container"
	"merionyx/api-gateway/internal/shared/grpcobs"
	"merionyx/api-gateway/internal/shared/serviceapp"
	authv1 "merionyx/api-gateway/pkg/api/auth/v1"
	environmentsv1 "merionyx/api-gateway/pkg/api/environments/v1"
	schemasv1 "merionyx/api-gateway/pkg/api/schemas/v1"
	snapshotsv1 "merionyx/api-gateway/pkg/api/snapshots/v1"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// RunGRPCServer serves the control-plane gRPC API until ctx is cancelled.
func RunGRPCServer(ctx context.Context, container *container.Container) error {
	lis, err := net.Listen("tcp", ":"+container.Config.Server.GRPCPort)
	if err != nil {
		return fmt.Errorf("failed to listen on :%s: %w", container.Config.Server.GRPCPort, err)
	}

	opts, err := grpcobs.ServerOptions(&container.Config.GRPCControlPlane.TLS, container.Config.GRPCControlPlane.Observability, container.Config.MetricsHTTP.Enabled)
	if err != nil {
		return fmt.Errorf("control-plane gRPC options: %w", err)
	}
	grpcSrv := grpc.NewServer(opts...)

	snapshotsv1.RegisterSnapshotsServiceServer(grpcSrv, container.SnapshotGRPCHandler)
	environmentsv1.RegisterEnvironmentsServiceServer(grpcSrv, container.EnvironmentsGRPCHandler)
	schemasv1.RegisterSchemasServiceServer(grpcSrv, container.SchemasGRPCHandler)
	authv1.RegisterAuthServiceServer(grpcSrv, container.AuthGRPCHandler)

	if container.Config.GRPCControlPlane.Observability.ReflectionEnabled {
		reflection.Register(grpcSrv)
	}

	slog.Info("gRPC server starting", "port", container.Config.Server.GRPCPort)
	return serviceapp.RunGRPCServeUntil(ctx, grpcSrv, lis)
}
