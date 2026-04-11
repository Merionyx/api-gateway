package errmapping

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDomainRules_SampleMatchesAndHTTPGRPCAgree(t *testing.T) {
	t.Parallel()
	for _, r := range DomainRules() {
		if r.Sample == nil {
			t.Fatalf("rule %q: missing Sample", r.Name)
		}
		if !r.Match(r.Sample) {
			t.Fatalf("rule %q: Sample does not match Match()", r.Name)
		}
		wrapped := fmt.Errorf("wrap: %w", r.Sample)
		if !r.Match(wrapped) {
			t.Fatalf("rule %q: wrapped Sample does not match", r.Name)
		}

		httpSt, probCode, detail := ResolveDomainProblem(wrapped)
		if httpSt != r.HTTPStatus {
			t.Fatalf("rule %q: HTTP %d want %d", r.Name, httpSt, r.HTTPStatus)
		}
		if probCode != r.ProblemCode {
			t.Fatalf("rule %q: problem code %q want %q", r.Name, probCode, r.ProblemCode)
		}
		if detail != r.Detail(wrapped) {
			t.Fatalf("rule %q: detail mismatch", r.Name)
		}

		gc, gmsg := GRPCStatus(wrapped)
		if gc != r.GRPC {
			t.Fatalf("rule %q: gRPC code %v want %v", r.Name, gc, r.GRPC)
		}
		if gmsg != r.Detail(wrapped) {
			t.Fatalf("rule %q: gRPC message must equal Problem detail", r.Name)
		}
	}
}

func TestGRPCStatus_UnmappedUsesInternalAndErrError(t *testing.T) {
	t.Parallel()
	err := errors.New("opaque failure")
	c, msg := GRPCStatus(err)
	if c != codes.Internal || msg != err.Error() {
		t.Fatalf("got %v %q", c, msg)
	}
}

func TestResolveDomainProblem_Unmapped(t *testing.T) {
	t.Parallel()
	err := errors.New("unknown")
	st, code, detail := ResolveDomainProblem(err)
	if st != http.StatusInternalServerError || code != CodeInternalError || detail != DetailInternalError {
		t.Fatalf("got %d %q %q", st, code, detail)
	}
}

func TestContractPipelineRules_Default(t *testing.T) {
	t.Parallel()
	err := errors.New("upstream timeout")
	st, code, detail := ResolveContractPipelineProblem(err)
	if st != http.StatusBadGateway || code != CodeContractSyncPipelineFailed || detail != DetailContractSyncPipelineFailed {
		t.Fatalf("got %d %q %q", st, code, detail)
	}
}

func TestGRPCStatus_MatchesProblemDelivery(t *testing.T) {
	t.Parallel()
	// Ensures grpcerr.Status uses the same mapping as HTTP Problem detail text.
	err := fmt.Errorf("%w: etcd", apierrors.ErrStoreAccess)
	gc, gmsg := GRPCStatus(err)
	st, ok := status.FromError(status.Error(gc, gmsg))
	if !ok || st == nil {
		t.Fatal("status.FromError")
	}
	if st.Code() != gc || st.Message() != gmsg {
		t.Fatalf("grpc status round-trip: %v %q", st.Code(), st.Message())
	}
	_, probCode, det := ResolveDomainProblem(err)
	if probCode != CodeStoreUnavailable || det != gmsg {
		t.Fatalf("HTTP problem code/detail diverge from gRPC: %q vs %q", det, gmsg)
	}
}
