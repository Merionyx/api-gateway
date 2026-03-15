// internal/xds/builder/listener.go
package builder

import (
	"fmt"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"merionyx/api-gateway/control-plane/internal/domain/models"
)

func BuildListeners(env *models.Environment) []*listenerv3.Listener {
	listener := buildHTTPListener(env)
	return []*listenerv3.Listener{listener}
}

func buildHTTPListener(env *models.Environment) *listenerv3.Listener {
	manager := &hcmv3.HttpConnectionManager{
		CodecType:  hcmv3.HttpConnectionManager_AUTO,
		StatPrefix: fmt.Sprintf("ingress_http_%s", env.Name),
		RouteSpecifier: &hcmv3.HttpConnectionManager_Rds{
			Rds: &hcmv3.Rds{
				ConfigSource: &corev3.ConfigSource{
					ResourceApiVersion: corev3.ApiVersion_V3,
					ConfigSourceSpecifier: &corev3.ConfigSource_Ads{
						Ads: &corev3.AggregatedConfigSource{},
					},
				},
				RouteConfigName: env.Name + "_routes",
			},
		},
		HttpFilters: []*hcmv3.HttpFilter{{
			Name: "envoy.filters.http.router",
			ConfigType: &hcmv3.HttpFilter_TypedConfig{
				TypedConfig: mustMarshalAny(&routerv3.Router{}),
			},
		}},
	}

	pbst, err := anypb.New(manager)
	if err != nil {
		panic(err)
	}

	return &listenerv3.Listener{
		Name: fmt.Sprintf("listener_%s", env.Name),
		Address: &corev3.Address{
			Address: &corev3.Address_SocketAddress{
				SocketAddress: &corev3.SocketAddress{
					Address: "0.0.0.0",
					PortSpecifier: &corev3.SocketAddress_PortValue{
						PortValue: 10000,
					},
				},
			},
		},
		FilterChains: []*listenerv3.FilterChain{{
			Filters: []*listenerv3.Filter{{
				Name: "envoy.filters.network.http_connection_manager",
				ConfigType: &listenerv3.Filter_TypedConfig{
					TypedConfig: pbst,
				},
			}},
		}},
	}
}

func mustMarshalAny(m proto.Message) *anypb.Any {
	a, err := anypb.New(m)
	if err != nil {
		panic(err)
	}
	return a
}
