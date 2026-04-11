package apierrors

import (
	"errors"
	"fmt"
	"testing"
)

func TestJoinStore(t *testing.T) {
	t.Parallel()
	if got := JoinStore("op", nil); got != nil {
		t.Fatalf("nil err: got %v", got)
	}
	inner := errors.New("inner")
	joined := JoinStore("get", inner)
	if !errors.Is(joined, ErrStoreAccess) {
		t.Fatalf("errors.Is StoreAccess: %v", joined)
	}
	if !errors.Is(joined, inner) {
		t.Fatalf("errors.Is inner: %v", joined)
	}
}

func TestJoinContractSyncer(t *testing.T) {
	t.Parallel()
	if got := JoinContractSyncer("rpc", nil); got != nil {
		t.Fatalf("nil err: got %v", got)
	}
	inner := errors.New("timeout")
	joined := JoinContractSyncer("sync", inner)
	if !errors.Is(joined, ErrContractSyncerUnavailable) {
		t.Fatalf("errors.Is Unavailable: %v", joined)
	}
	if !errors.Is(joined, inner) {
		t.Fatalf("errors.Is inner: %v", joined)
	}
	if !errors.Is(fmt.Errorf("wrap: %w", joined), ErrContractSyncerUnavailable) {
		t.Fatalf("wrapped join should still match sentinel")
	}
}
