package grpcobs

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// ServerOptions returns grpc.ServerOption slice: TLS (if any), chained interceptors.
// recordPrometheus should match metrics_http.enabled so gRPC counters match the scrape endpoint.
func ServerOptions(tlsCfg *ServerTLSConfig, obs ObservabilityConfig, recordPrometheus bool) ([]grpc.ServerOption, error) {
	var out []grpc.ServerOption

	if tlsCfg != nil {
		tc, err := ServerTLS(*tlsCfg)
		if err != nil {
			return nil, err
		}
		if tc != nil {
			out = append(out, grpc.Creds(credentials.NewTLS(tc)))
		}
	}

	io := interceptorOpts{
		metrics: recordPrometheus,
		log:     obs.LogRequests,
	}
	out = append(out,
		grpc.ChainUnaryInterceptor(chainUnary(io)),
		grpc.ChainStreamInterceptor(chainStream(io)),
	)

	return out, nil
}

// MustServerOptions like ServerOptions but panics on TLS error — avoid in library; use for tests only if needed.
func MustServerOptions(tlsCfg *ServerTLSConfig, obs ObservabilityConfig, recordPrometheus bool) []grpc.ServerOption {
	opts, err := ServerOptions(tlsCfg, obs, recordPrometheus)
	if err != nil {
		panic(fmt.Errorf("grpcobs.ServerOptions: %w", err))
	}
	return opts
}
