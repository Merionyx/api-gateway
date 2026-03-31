package server

import (
	"fmt"
	"log"
	"merionyx/api-gateway/internal/controller/container"
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

	server := grpc.NewServer()

	// Register services
	snapshotsv1.RegisterSnapshotsServiceServer(server, container.SnapshotGRPCHandler)
	environmentsv1.RegisterEnvironmentsServiceServer(server, container.EnvironmentsGRPCHandler)
	schemasv1.RegisterSchemasServiceServer(server, container.SchemasGRPCHandler)
	authv1.RegisterAuthServiceServer(server, container.AuthGRPCHandler)

	reflection.Register(server)

	log.Printf("gRPC server starting on :%s", container.Config.Server.GRPCPort)
	return server.Serve(lis)
}
