package snapshot

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"merionyx/api-gateway/internal/controller/domain/interfaces"
	"merionyx/api-gateway/internal/controller/domain/models"
	"os"
	"sort"
	"time"

	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	envoycache "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
)

func BuildEnvoySnapshot(xdsBuilder interfaces.XDSBuilder, env *models.Environment) *envoycache.Snapshot {
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

	version, err := snapshotVersionFromResources(listenerResources, clusterResources, routeResources, endpointResources)
	if err != nil {
		slog.Warn("snapshot: stable version failed, using time-based version", "error", err)
		version = fmt.Sprintf("v%d", time.Now().Unix())
	}

	snapshot, err := envoycache.NewSnapshot(
		version,
		map[resource.Type][]types.Resource{
			resource.ListenerType: listenerResources,
			resource.ClusterType:  clusterResources,
			resource.RouteType:    routeResources,
			resource.EndpointType: endpointResources,
		},
	)
	if err != nil {
		slog.Error("failed to create envoy snapshot", "error", err)
		os.Exit(1)
	}

	return snapshot
}

func snapshotVersionFromResources(
	listenerResources, clusterResources, routeResources, endpointResources []types.Resource,
) (string, error) {
	h := sha256.New()
	groups := []struct {
		tag string
		res []types.Resource
	}{
		{"L", listenerResources},
		{"C", clusterResources},
		{"R", routeResources},
		{"E", endpointResources},
	}
	for _, g := range groups {
		h.Write([]byte(g.tag))
		part, err := hashResourceList(g.res)
		if err != nil {
			return "", err
		}
		h.Write(part)
	}
	sum := h.Sum(nil)
	return fmt.Sprintf("v%x", sum[:16]), nil
}

func hashResourceList(resources []types.Resource) ([]byte, error) {
	if len(resources) == 0 {
		return []byte{0}, nil
	}
	idx := make([]int, len(resources))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(i, j int) bool {
		a := envoycache.GetResourceName(resources[idx[i]])
		b := envoycache.GetResourceName(resources[idx[j]])
		if a != b {
			return a < b
		}
		return idx[i] < idx[j]
	})
	inner := sha256.New()
	for _, i := range idx {
		b, err := envoycache.MarshalResource(resources[i])
		if err != nil {
			return nil, err
		}
		inner.Write([]byte(envoycache.GetResourceName(resources[i])))
		inner.Write([]byte{0})
		inner.Write(b)
	}
	return inner.Sum(nil), nil
}
