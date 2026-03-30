package snapshot

import (
	"fmt"
	"log"
	"merionyx/api-gateway/control-plane/internal/domain/models"
	"merionyx/api-gateway/control-plane/internal/xds/builder"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

func BuildEnvoySnapshot(xdsBuilder *builder.XDSBuilder, env *models.Environment) *cache.Snapshot {
	version := fmt.Sprintf("v%d", time.Now().Unix())

	listeners := xdsBuilder.BuildListeners(env)
	clusters := xdsBuilder.BuildClusters(env)
	routes := xdsBuilder.BuildRoutes(env)
	endpoints := xdsBuilder.BuildEndpoints(env)

	listenerResources := make([]types.Resource, len(listeners))
	for i, l := range listeners {
		listenerResources[i] = l
	}

	clusterResources := make([]types.Resource, len(clusters))
	for i, c := range clusters {
		clusterResources[i] = c
	}

	routeResources := make([]types.Resource, len(routes))
	for i, r := range routes {
		routeResources[i] = r
	}

	endpointResources := make([]types.Resource, len(endpoints))
	for i, e := range endpoints {
		endpointResources[i] = e
	}

	snapshot, err := cache.NewSnapshot(
		version,
		map[resource.Type][]types.Resource{
			resource.ListenerType: listenerResources,
			resource.ClusterType:  clusterResources,
			resource.RouteType:    routeResources,
			resource.EndpointType: endpointResources,
		},
	)
	if err != nil {
		log.Fatalf("Failed to create snapshot: %v", err)
	}

	return snapshot
}
