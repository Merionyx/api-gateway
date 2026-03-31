package builder

import (
	"strconv"
	"strings"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	upstreamhttpv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"

	"merionyx/api-gateway/internal/controller/domain/models"
)

func (b *xdsBuilder) BuildClusters(env *models.Environment) []*clusterv3.Cluster {
	clusters := make([]*clusterv3.Cluster, 0)
	uniqueServices := make(map[string]string)

	// 1. Добавляем Auth Sidecar кластер
	authSidecarCluster := buildAuthSidecarCluster()
	clusters = append(clusters, authSidecarCluster)

	// 2. Добавляем сервисы из environment
	for _, service := range env.Services.Static {
		uniqueServices[service.Name] = service.Upstream
	}

	// 3. Добавляем глобальные сервисы
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

	// 4. Создаем кластеры для сервисов
	for serviceName, upstream := range uniqueServices {
		cluster := buildCluster(serviceName, upstream)
		clusters = append(clusters, cluster)
	}

	return clusters
}

// buildAuthSidecarCluster создает кластер для Auth Sidecar
func buildAuthSidecarCluster() *clusterv3.Cluster {
	return &clusterv3.Cluster{
		Name:           "auth_sidecar",
		ConnectTimeout: durationpb.New(1 * time.Second),

		// Используем LOGICAL_DNS для резолва hostname
		ClusterDiscoveryType: &clusterv3.Cluster_Type{
			Type: clusterv3.Cluster_LOGICAL_DNS,
		},

		// DNS lookup только для IPv4
		DnsLookupFamily: clusterv3.Cluster_V4_ONLY,

		LoadAssignment: &endpointv3.ClusterLoadAssignment{
			ClusterName: "auth_sidecar",
			Endpoints: []*endpointv3.LocalityLbEndpoints{{
				LbEndpoints: []*endpointv3.LbEndpoint{{
					HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
						Endpoint: &endpointv3.Endpoint{
							Address: &corev3.Address{
								Address: &corev3.Address_SocketAddress{
									SocketAddress: &corev3.SocketAddress{
										Address: "auth-sidecar-dev", // Hostname (будет резолвиться через DNS)
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

		// HTTP/2 для gRPC (новый способ)
		TypedExtensionProtocolOptions: map[string]*anypb.Any{
			"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": mustMarshalAny(
				&upstreamhttpv3.HttpProtocolOptions{
					UpstreamProtocolOptions: &upstreamhttpv3.HttpProtocolOptions_ExplicitHttpConfig_{
						ExplicitHttpConfig: &upstreamhttpv3.HttpProtocolOptions_ExplicitHttpConfig{
							ProtocolConfig: &upstreamhttpv3.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{
								Http2ProtocolOptions: &corev3.Http2ProtocolOptions{},
							},
						},
					},
				},
			),
		},
	}
}

func buildCluster(name, upstream string) *clusterv3.Cluster {
	host, port := parseUpstream(upstream)

	return &clusterv3.Cluster{
		Name:           name,
		ConnectTimeout: durationpb.New(5 * time.Second),
		ClusterDiscoveryType: &clusterv3.Cluster_Type{
			Type: clusterv3.Cluster_LOGICAL_DNS,
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
