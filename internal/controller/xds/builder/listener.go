package builder

import (
	"fmt"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	extauthzv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/ext_authz/v3"
	routerv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	hcmv3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	typev3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/merionyx/api-gateway/internal/controller/domain/models"
)

func (b *xdsBuilder) BuildListeners(env *models.Environment) ([]*listenerv3.Listener, error) {
	listener, err := buildHTTPListener(env)
	if err != nil {
		return nil, err
	}
	return []*listenerv3.Listener{listener}, nil
}

func buildHTTPListener(env *models.Environment) (*listenerv3.Listener, error) {
	httpFilters, err := buildHTTPFilters()
	if err != nil {
		return nil, err
	}
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
		HttpFilters: httpFilters,

		RequestTimeout: durationpb.New(30 * time.Second),
		// Tracing: uses the global HTTP tracer from Envoy bootstrap (OpenTelemetry OTLP) when
		// the edge chart enables envoy.tracing; 100% sampling here is independent of auth app sampling.
		// Spawn upstream span for a gateway / edge role (W3C propagation to clusters).
		Tracing: &hcmv3.HttpConnectionManager_Tracing{
			ClientSampling:    &typev3.Percent{Value: 100},
			RandomSampling:    &typev3.Percent{Value: 100},
			OverallSampling:   &typev3.Percent{Value: 100},
			SpawnUpstreamSpan: wrapperspb.Bool(true),
		},
	}
	pbst, err := anypb.New(manager)
	if err != nil {
		return nil, fmt.Errorf("marshal HttpConnectionManager: %w", err)
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
	}, nil
}

func buildHTTPFilters() ([]*hcmv3.HttpFilter, error) {
	extAuthz, err := buildExtAuthzFilter()
	if err != nil {
		return nil, err
	}
	router, err := buildRouterFilter()
	if err != nil {
		return nil, err
	}
	return []*hcmv3.HttpFilter{extAuthz, router}, nil
}

func buildExtAuthzFilter() (*hcmv3.HttpFilter, error) {
	extAuthz := &extauthzv3.ExtAuthz{
		Services: &extauthzv3.ExtAuthz_GrpcService{
			GrpcService: &corev3.GrpcService{
				TargetSpecifier: &corev3.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &corev3.GrpcService_EnvoyGrpc{
						ClusterName: "auth_sidecar",
					},
				},
				Timeout: durationpb.New(1 * time.Second),
			},
		},

		FailureModeAllow: false,

		WithRequestBody: &extauthzv3.BufferSettings{
			MaxRequestBytes:     8192,
			AllowPartialMessage: true,
		},

		ClearRouteCache: true,
	}
	anyExt, err := anypb.New(extAuthz)
	if err != nil {
		return nil, fmt.Errorf("marshal ext_authz: %w", err)
	}
	return &hcmv3.HttpFilter{
		Name: "envoy.filters.http.ext_authz",
		ConfigType: &hcmv3.HttpFilter_TypedConfig{
			TypedConfig: anyExt,
		},
	}, nil
}

func buildRouterFilter() (*hcmv3.HttpFilter, error) {
	anyRouter, err := anypb.New(&routerv3.Router{
		SuppressEnvoyHeaders: false,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal router: %w", err)
	}
	return &hcmv3.HttpFilter{
		Name: "envoy.filters.http.router",
		ConfigType: &hcmv3.HttpFilter_TypedConfig{
			TypedConfig: anyRouter,
		},
	}, nil
}
