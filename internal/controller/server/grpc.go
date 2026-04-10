package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	authv1 "github.com/merionyx/api-gateway/pkg/grpc/auth/v1"
	environmentsv1 "github.com/merionyx/api-gateway/pkg/grpc/environments/v1"
	schemasv1 "github.com/merionyx/api-gateway/pkg/grpc/schemas/v1"
	snapshotsv1 "github.com/merionyx/api-gateway/pkg/grpc/snapshots/v1"

	"github.com/merionyx/api-gateway/internal/controller/container"
	"github.com/merionyx/api-gateway/internal/shared/grpcobs"
	"github.com/merionyx/api-gateway/internal/shared/serviceapp"

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
