package builder

import (
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

func (b *xdsBuilder) BuildEndpoints(env *models.Environment) ([]*endpointv3.ClusterLoadAssignment, error) {
	endpoints := make([]*endpointv3.ClusterLoadAssignment, 0)

	for _, service := range env.Services.Static {
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

	return endpoints, nil
}
