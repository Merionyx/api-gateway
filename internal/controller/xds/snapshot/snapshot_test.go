package snapshot

import (
	"testing"

	"merionyx/api-gateway/internal/controller/domain/models"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routev3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
)

type stubXDSBuilder struct{}

func (stubXDSBuilder) BuildListeners(*models.Environment) []*listenerv3.Listener   { return nil }
func (stubXDSBuilder) BuildClusters(*models.Environment) []*clusterv3.Cluster      { return nil }
func (stubXDSBuilder) BuildRoutes(*models.Environment) []*routev3.RouteConfiguration { return nil }
func (stubXDSBuilder) BuildEndpoints(*models.Environment) []*endpointv3.ClusterLoadAssignment {
	return nil
}

func TestBuildEnvoySnapshot_EmptyResources(t *testing.T) {
	env := &models.Environment{Name: "e1"}
	snap, err := BuildEnvoySnapshot(stubXDSBuilder{}, env)
	if err != nil {
		t.Fatal(err)
	}
	if snap == nil {
		t.Fatal("nil snapshot")
	}
}
