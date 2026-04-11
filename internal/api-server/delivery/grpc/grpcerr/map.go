// Package grpcerr maps domain errors to gRPC status codes.
package grpcerr

import (
	"errors"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Status converts a domain error to a gRPC status. Nil error returns nil.
// Unknown errors use codes.Internal.
func Status(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, apierrors.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, apierrors.ErrContractSyncerRejected):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
