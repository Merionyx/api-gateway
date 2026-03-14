package servers

import (
	"fmt"
	"log"
	"merionyx/api-gateway/control-plane/internal/container"
	environmentv1 "merionyx/api-gateway/control-plane/pkg/api/environment/v1"
	listenerv1 "merionyx/api-gateway/control-plane/pkg/api/listener/v1"
	tenantv1 "merionyx/api-gateway/control-plane/pkg/api/tenant/v1"
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
	tenantv1.RegisterTenantServiceServer(server, container.TenantGRPCHandler)
	environmentv1.RegisterEnvironmentServiceServer(server, container.EnvironmentGRPCHandler)
	listenerv1.RegisterListenerServiceServer(server, container.ListenerGRPCHandler)

	reflection.Register(server)

	log.Printf("gRPC server starting on :9090")
	return server.Serve(lis)
}
