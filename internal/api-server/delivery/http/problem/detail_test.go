package problem

import (
	"errors"
	"fmt"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

func TestDetailContractSyncRejected(t *testing.T) {
	t.Parallel()
	reason := "invalid bundle name"
	err := fmt.Errorf("%w: %s", apierrors.ErrContractSyncerRejected, reason)
	if got := DetailContractSyncRejected(err); got != reason {
		t.Fatalf("got %q want %q", got, reason)
	}
	if got := DetailContractSyncRejected(errors.New("other")); got != DetailContractSyncerRejected {
		t.Fatalf("got %q", got)
	}
}

func TestTypeURI(t *testing.T) {
	t.Parallel()
	if got, want := TypeURI("NOT_FOUND"), ProblemsDocBase+"#NOT_FOUND"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
