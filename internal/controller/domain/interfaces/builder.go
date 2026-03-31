package interfaces

import (
	"merionyx/api-gateway/internal/controller/domain/models"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
)

type XDSBuilder interface {
	BuildListeners(env *models.Environment) []*listenerv3.Listener
	BuildClusters(env *models.Environment) []*clusterv3.Cluster
	BuildRoutes(env *models.Environment) []*routev3.RouteConfiguration
	BuildEndpoints(env *models.Environment) []*endpointv3.ClusterLoadAssignment
}
