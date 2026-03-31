package server

import (
	"fmt"
	"log"
	"merionyx/api-gateway/internal/api-server/container"
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

	reflection.Register(server)

	log.Printf("gRPC server starting on :%s", container.Config.Server.GRPCPort)
	return server.Serve(lis)
}
