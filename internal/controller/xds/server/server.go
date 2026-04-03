package server

import (
	"context"
	"fmt"
	"log/slog"
	"merionyx/api-gateway/internal/controller/xds"
	"net"

	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"

	// xDS gRPC service definitions
	clusterservice "github.com/envoyproxy/go-control-plane/envoy/service/cluster/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	endpointservice "github.com/envoyproxy/go-control-plane/envoy/service/endpoint/v3"
	listenerservice "github.com/envoyproxy/go-control-plane/envoy/service/listener/v3"
	routeservice "github.com/envoyproxy/go-control-plane/envoy/service/route/v3"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// internal/xds/server/server.go
type Server struct {
	grpcServer *grpc.Server
	xdsServer  server.Server
	cache      cache.SnapshotCache
}

func NewXDSServer(snapshotCache cache.SnapshotCache, port int) *Server {
	grpcServer := grpc.NewServer()

	cb := &xds.Callbacks{}
	xdsServer := server.NewServer(context.Background(), snapshotCache, cb)

	// xDS services
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, xdsServer)
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, xdsServer)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, xdsServer)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, xdsServer)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, xdsServer)

	reflection.Register(grpcServer)

	return &Server{
		grpcServer: grpcServer,
		xdsServer:  xdsServer,
		cache:      snapshotCache,
	}
}

func (s *Server) Start(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	slog.Info("xDS server listening", "port", port)
	return s.grpcServer.Serve(lis)
}
func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
}
