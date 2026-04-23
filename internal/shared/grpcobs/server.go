package grpcobs

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// ServerOptions returns grpc.ServerOption slice: TLS (if any), chained interceptors
// (metrics, optional request logging). It does not add JWT, API key, or OIDC auth (roadmap ш.27).
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
