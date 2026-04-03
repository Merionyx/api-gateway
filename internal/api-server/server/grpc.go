package server

import (
	"fmt"
	"log/slog"
	"merionyx/api-gateway/internal/api-server/container"
	pb "merionyx/api-gateway/pkg/api/controller_registry/v1"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func StartGRPCServer(cnt *container.Container) error {
	address := fmt.Sprintf("%s:%s", cnt.Config.Server.Host, cnt.Config.Server.GRPCPort)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	server := grpc.NewServer()

	pb.RegisterControllerRegistryServiceServer(server, cnt.ControllerRegistryHandler)

	reflection.Register(server)

	slog.Info("gRPC server starting", "address", address)
	return server.Serve(lis)
}
