package builder

import (
	"strconv"
	"strings"
	"time"

	clusterv3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	"google.golang.org/protobuf/types/known/durationpb"

	"merionyx/api-gateway/control-plane/internal/domain/models"
	"merionyx/api-gateway/control-plane/internal/repository/memory"
)

func BuildClusters(env *models.Environment, serviceRepo *memory.ServiceRepository) []*clusterv3.Cluster {
	clusters := make([]*clusterv3.Cluster, 0)
	uniqueServices := make(map[string]string)
	// 1. Добавляем сервисы из environment
	for _, service := range env.Services.Static {
		uniqueServices[service.Name] = service.Upstream
	}
	// 2. Добавляем глобальные сервисы
	if serviceRepo != nil {
		globalServices, err := serviceRepo.ListServices()
		if err == nil {
			for _, service := range globalServices {
				// Environment-specific services override global ones
				if _, exists := uniqueServices[service.Name]; !exists {
					uniqueServices[service.Name] = service.Upstream
				}
			}
		}
	}
	for serviceName, upstream := range uniqueServices {
		cluster := buildCluster(serviceName, upstream)
		clusters = append(clusters, cluster)
	}
	return clusters
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
