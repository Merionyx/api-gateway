package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
	"github.com/merionyx/api-gateway/internal/shared/grpcobs"
)

func TestClient_Ping_emptyAddress(t *testing.T) {
	t.Parallel()
	c := NewClient("", grpcobs.ClientTLSConfig{})
	err := c.Ping(context.Background())
	if err == nil || !errors.Is(err, apierrors.ErrInvalidInput) {
		t.Fatalf("got %v", err)
	}
}
