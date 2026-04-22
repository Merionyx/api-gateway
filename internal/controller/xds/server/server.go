package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	xdspkg "github.com/merionyx/api-gateway/internal/controller/xds"
	"github.com/merionyx/api-gateway/internal/shared/serviceapp"

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

func NewXDSServer(snapshotCache cache.SnapshotCache, registerReflection bool, metricsEnabled bool, xdsTraceCallbacks bool, opts ...grpc.ServerOption) *Server {
	grpcServer := grpc.NewServer(opts...)

	cb := xdspkg.NewCallbacks(metricsEnabled, xdsTraceCallbacks)
	xdsServer := server.NewServer(context.Background(), snapshotCache, cb)

	// xDS services
	discoverygrpc.RegisterAggregatedDiscoveryServiceServer(grpcServer, xdsServer)
	endpointservice.RegisterEndpointDiscoveryServiceServer(grpcServer, xdsServer)
	clusterservice.RegisterClusterDiscoveryServiceServer(grpcServer, xdsServer)
	routeservice.RegisterRouteDiscoveryServiceServer(grpcServer, xdsServer)
	listenerservice.RegisterListenerDiscoveryServiceServer(grpcServer, xdsServer)

	if registerReflection {
		reflection.Register(grpcServer)
	}

	return &Server{
		grpcServer: grpcServer,
		xdsServer:  xdsServer,
		cache:      snapshotCache,
	}
}

func (s *Server) Start(port int) error {
	return s.Run(context.Background(), port)
}

// Run serves xDS until ctx is cancelled, then stops gracefully.
func (s *Server) Run(ctx context.Context, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	slog.Info("xDS server listening", "port", port)
	return serviceapp.RunGRPCServeUntil(ctx, s.grpcServer, lis)
}

func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
}
