// internal/xds/builder/endpoint.go
package builder

import (
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"

	"merionyx/api-gateway/control-plane/internal/domain/models"
)

func BuildEndpoints(env *models.Environment) []*endpointv3.ClusterLoadAssignment {
	endpoints := make([]*endpointv3.ClusterLoadAssignment, 0)

	// Для каждого сервиса создаём endpoint
	for _, service := range env.Services.List {
		host, port := parseUpstream(service.Upstream)

		endpoint := &endpointv3.ClusterLoadAssignment{
			ClusterName: service.Name,
			Endpoints: []*endpointv3.LocalityLbEndpoints{{
				LbEndpoints: []*endpointv3.LbEndpoint{{
					HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
						Endpoint: &endpointv3.Endpoint{
							Address: &corev3.Address{
								Address: &corev3.Address_SocketAddress{
									SocketAddress: &corev3.SocketAddress{
										Address: host,
										PortSpecifier: &corev3.SocketAddress_PortValue{
											PortValue: uint32(port),
										},
									},
								},
							},
						},
					},
				}},
			}},
		}

		endpoints = append(endpoints, endpoint)
	}

	return endpoints
}
