package problem

import (
	"errors"
	"net/http"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"
)

func TestFromDomain(t *testing.T) {
	t.Parallel()
	st, _ := FromDomain(nil)
	if st != http.StatusOK {
		t.Fatalf("nil err: status %d", st)
	}

	st, _ = FromDomain(apierrors.ErrNotFound)
	if st != http.StatusNotFound {
		t.Fatalf("status %d", st)
	}

	st, _ = FromDomain(errors.New("opaque"))
	if st != http.StatusInternalServerError {
		t.Fatalf("status %d", st)
	}
}

func TestFromContractSyncPipeline(t *testing.T) {
	t.Parallel()
	st, _ := FromContractSyncPipeline(nil)
	if st != http.StatusOK {
		t.Fatalf("nil: %d", st)
	}

	st, _ = FromContractSyncPipeline(errors.New("upstream"))
	if st != http.StatusBadGateway {
		t.Fatalf("status %d", st)
	}
}
