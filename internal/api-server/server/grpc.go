// Package server hosts API Server HTTP (Fiber) and gRPC (controller registry).
//
// MVP split (roadmap п.8, ш.27): interactive JWT, API keys, and OIDC callbacks are HTTP-only
// (see RunHTTPServer). RunGRPCServer uses transport TLS/mTLS from grpc_registry plus grpcobs
// metrics/logging only—no per-RPC Bearer or API-key middleware unless explicitly redesigned.
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	pb "github.com/merionyx/api-gateway/pkg/grpc/controller_registry/v1"

	"github.com/merionyx/api-gateway/internal/api-server/container"
	"github.com/merionyx/api-gateway/internal/shared/grpcobs"
	"github.com/merionyx/api-gateway/internal/shared/serviceapp"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// RunGRPCServer serves the controller-registry gRPC API until ctx is cancelled.
func RunGRPCServer(ctx context.Context, cnt *container.Container) error {
	address := fmt.Sprintf("%s:%s", cnt.Config.Server.Host, cnt.Config.Server.GRPCPort)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	opts, err := grpcobs.ServerOptions(&cnt.Config.GRPCRegistry.TLS, cnt.Config.GRPCRegistry.Observability, cnt.Config.MetricsHTTP.Enabled)
	if err != nil {
		return fmt.Errorf("gRPC registry options: %w", err)
	}
	grpcSrv := grpc.NewServer(opts...)

	pb.RegisterControllerRegistryServiceServer(grpcSrv, cnt.ControllerRegistryHandler)

	if cnt.Config.GRPCRegistry.Observability.ReflectionEnabled {
		reflection.Register(grpcSrv)
	}

	slog.Info("gRPC server starting", "address", address)
	return serviceapp.RunGRPCServeUntil(ctx, grpcSrv, lis)
}
