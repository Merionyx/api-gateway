// Package grpcerr maps domain errors to gRPC status codes.
package grpcerr

import (
	"github.com/merionyx/api-gateway/internal/api-server/domain/errmapping"
	"github.com/merionyx/api-gateway/internal/api-server/metrics"

	"google.golang.org/grpc/status"
)

// Status converts a domain error to a gRPC status using the same rules as HTTP Problem (errmapping).
// Nil error returns nil. Unmapped errors use codes.Internal with err.Error().
// When metricsEnabled, increments api_server_domain_errors_total by domain outcome.
func Status(metricsEnabled bool, err error) error {
	if err == nil {
		return nil
	}
	metrics.RecordDomainOutcome(metricsEnabled, metrics.TransportGRPC, err)
	c, msg := errmapping.GRPCStatus(err)
	return status.Error(c, msg)
}
