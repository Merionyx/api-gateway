package usecase

import (
	"testing"

	"merionyx/api-gateway/internal/shared/grpcobs"
)

func TestBundleSyncUseCase_grpcDialOptions_Insecure(t *testing.T) {
	uc := NewBundleSyncUseCase(nil, nil, "127.0.0.1:1", grpcobs.ClientTLSConfig{}, nil, false)
	opts, err := uc.grpcDialOptions()
	if err != nil {
		t.Fatal(err)
	}
	if len(opts) == 0 {
		t.Fatal("expected dial options")
	}
}
