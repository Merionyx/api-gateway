package grpc

import (
	"testing"

	"github.com/merionyx/api-gateway/internal/shared/grpcobs"
)

func TestDialOptions_Insecure(t *testing.T) {
	opts, err := DialOptions(grpcobs.ClientTLSConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if len(opts) == 0 {
		t.Fatal("expected dial options")
	}
}
