package builder

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	upstreamhttpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

func (b *xdsBuilder) BuildClusters(env *models.Environment) ([]*clusterv3.Cluster, error) {
	clusters := make([]*clusterv3.Cluster, 0)
	uniqueServices := make(map[string]string)

	authSidecarCluster, err := buildAuthSidecarCluster()
	if err != nil {
		return nil, err
	}
	clusters = append(clusters, authSidecarCluster)

	// 2. Add services from environment
	for _, service := range env.Services.Static {
		uniqueServices[service.Name] = service.Upstream
	}

	// 3. Add global services
	if b.inMemoryServiceRepository != nil {
		globalServices, err := b.inMemoryServiceRepository.ListServices()
		if err == nil {
			for _, service := range globalServices {
				if _, exists := uniqueServices[service.Name]; !exists {
					uniqueServices[service.Name] = service.Upstream
				}
			}
		}
	}

	// 4. Create clusters for services
	for serviceName, upstream := range uniqueServices {
		cluster := buildCluster(serviceName, upstream)
		clusters = append(clusters, cluster)
	}

	return clusters, nil
}

func buildAuthSidecarCluster() (*clusterv3.Cluster, error) {
	httpProtoOpts, err := anypb.New(
		&upstreamhttpv3.HttpProtocolOptions{
			UpstreamProtocolOptions: &upstreamhttpv3.HttpProtocolOptions_ExplicitHttpConfig_{
				ExplicitHttpConfig: &upstreamhttpv3.HttpProtocolOptions_ExplicitHttpConfig{
					ProtocolConfig: &upstreamhttpv3.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{
						Http2ProtocolOptions: &corev3.Http2ProtocolOptions{},
					},
				},
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("marshal auth sidecar HttpProtocolOptions: %w", err)
	}
	return &clusterv3.Cluster{
		Name:           "auth_sidecar",
		ConnectTimeout: durationpb.New(1 * time.Second),

		// Envoy and auth-sidecar run in the same pod (edge chart): gRPC ext_authz to loopback, not a K8s Service name.
		ClusterDiscoveryType: &clusterv3.Cluster_Type{
			Type: clusterv3.Cluster_STATIC,
		},

		LoadAssignment: &endpointv3.ClusterLoadAssignment{
			ClusterName: "auth_sidecar",
			Endpoints: []*endpointv3.LocalityLbEndpoints{{
				LbEndpoints: []*endpointv3.LbEndpoint{{
					HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
						Endpoint: &endpointv3.Endpoint{
							Address: &corev3.Address{
								Address: &corev3.Address_SocketAddress{
									SocketAddress: &corev3.SocketAddress{
										Address: "127.0.0.1",
										PortSpecifier: &corev3.SocketAddress_PortValue{
											PortValue: 9001,
										},
									},
								},
							},
						},
					},
				}},
			}},
		},

		TypedExtensionProtocolOptions: map[string]*anypb.Any{
			"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": httpProtoOpts,
		},
	}, nil
}

func buildCluster(name, upstream string) *clusterv3.Cluster {
	host, port := parseUpstream(upstream)

	return &clusterv3.Cluster{
		Name:           name,
		ConnectTimeout: durationpb.New(5 * time.Second),
		ClusterDiscoveryType: &clusterv3.Cluster_Type{
			Type: clusterv3.Cluster_STRICT_DNS,
		},
		DnsLookupFamily: clusterv3.Cluster_V4_ONLY,
		LoadAssignment: &endpointv3.ClusterLoadAssignment{
			ClusterName: name,
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
		},
	}
}

func parseUpstream(upstream string) (string, int) {
	upstream = strings.TrimPrefix(upstream, "http://")
	upstream = strings.TrimPrefix(upstream, "https://")

	parts := strings.Split(upstream, ":")
	if len(parts) == 2 {
		port, _ := strconv.Atoi(parts[1])
		return parts[0], port
	}

	return upstream, 80
}
