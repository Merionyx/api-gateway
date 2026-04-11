package grpcerr

import (
	"errors"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestStatus(t *testing.T) {
	t.Parallel()
	if err := Status(false, nil); err != nil {
		t.Fatalf("nil: %v", err)
	}
	if err := Status(false, errors.New("opaque")); err == nil {
		t.Fatal("expected status error")
	}

	err := Status(false, apierrors.ErrNotFound)
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.NotFound {
		t.Fatalf("got %v", st)
	}

	err = Status(true, errors.New("unmapped grpcerr"))
	st, ok = status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("got %v", st)
	}
}
