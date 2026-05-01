package errmapping

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/merionyx/api-gateway/internal/api-server/domain/apierrors"

	"google.golang.org/grpc/codes"
)

// Rule maps a domain predicate to HTTP status, gRPC code, and Problem payload (code + detail).
// First matching rule wins — keep order aligned with historical switch statements.
// Sample is a concrete error that must satisfy Match (for tests and documentation).
type Rule struct {
	Name        string
	Sample      error
	Match       func(error) bool
	HTTPStatus  int
	GRPC        codes.Code
	ProblemCode string
	Detail      func(error) string
}

func staticDetail(s string) func(error) string {
	return func(error) string { return s }
}

// DomainRules is the canonical list for apierrors sentinels (HTTP + Problem + gRPC).
func DomainRules() []Rule {
	return []Rule{
		{
			Name:        "NotFound",
			Sample:      apierrors.ErrNotFound,
			Match:       func(e error) bool { return errors.Is(e, apierrors.ErrNotFound) },
			HTTPStatus:  http.StatusNotFound,
			GRPC:        codes.NotFound,
			ProblemCode: CodeNotFound,
			Detail:      staticDetail(DetailNotFound),
		},
		{
			Name:        "InvalidInput",
			Sample:      apierrors.ErrInvalidInput,
			Match:       func(e error) bool { return errors.Is(e, apierrors.ErrInvalidInput) },
			HTTPStatus:  http.StatusBadRequest,
			GRPC:        codes.InvalidArgument,
			ProblemCode: CodeInvalidInput,
			Detail:      staticDetail(DetailInvalidInput),
		},
		{
			Name:        "ContractSyncerRejected",
			Sample:      fmt.Errorf("%w: upstream", apierrors.ErrContractSyncerRejected),
			Match:       func(e error) bool { return errors.Is(e, apierrors.ErrContractSyncerRejected) },
			HTTPStatus:  http.StatusBadRequest,
			GRPC:        codes.InvalidArgument,
			ProblemCode: CodeContractSyncerRejected,
			Detail:      func(e error) string { return DetailContractSyncRejected(e) },
		},
		{
			Name:        "NoActiveSigningKey",
			Sample:      apierrors.ErrNoActiveSigningKey,
			Match:       func(e error) bool { return errors.Is(e, apierrors.ErrNoActiveSigningKey) },
			HTTPStatus:  http.StatusServiceUnavailable,
			GRPC:        codes.Unavailable,
			ProblemCode: CodeNoActiveSigningKey,
			Detail:      staticDetail(DetailNoActiveSigningKey),
		},
		{
			Name:        "UnsupportedSigningAlgorithm",
			Sample:      apierrors.ErrUnsupportedSigningAlgorithm,
			Match:       func(e error) bool { return errors.Is(e, apierrors.ErrUnsupportedSigningAlgorithm) },
			HTTPStatus:  http.StatusInternalServerError,
			GRPC:        codes.Internal,
			ProblemCode: CodeUnsupportedSigningAlgorithm,
			Detail:      staticDetail(DetailUnsupportedSigningAlgorithm),
		},
		{
			Name:        "SigningOperationFailed",
			Sample:      apierrors.ErrSigningOperationFailed,
			Match:       func(e error) bool { return errors.Is(e, apierrors.ErrSigningOperationFailed) },
			HTTPStatus:  http.StatusInternalServerError,
			GRPC:        codes.Internal,
			ProblemCode: CodeSigningOperationFailed,
			Detail:      staticDetail(DetailSigningOperationFailed),
		},
		{
			Name:        "StoreAccess",
			Sample:      fmt.Errorf("%w: etcd", apierrors.ErrStoreAccess),
			Match:       func(e error) bool { return errors.Is(e, apierrors.ErrStoreAccess) },
			HTTPStatus:  http.StatusServiceUnavailable,
			GRPC:        codes.Unavailable,
			ProblemCode: CodeStoreUnavailable,
			Detail:      staticDetail(DetailStoreUnavailable),
		},
		{
			Name:        "ContractSyncerUnavailable",
			Sample:      fmt.Errorf("%w: dial", apierrors.ErrContractSyncerUnavailable),
			Match:       func(e error) bool { return errors.Is(e, apierrors.ErrContractSyncerUnavailable) },
			HTTPStatus:  http.StatusBadGateway,
			GRPC:        codes.Unavailable,
			ProblemCode: CodeContractSyncerUnavailable,
			Detail:      staticDetail(DetailContractSyncerUnavailable),
		},
		{
			Name:        "SessionRefreshConflict",
			Sample:      fmt.Errorf("%w", apierrors.ErrSessionRefreshConflict),
			Match:       func(e error) bool { return errors.Is(e, apierrors.ErrSessionRefreshConflict) },
			HTTPStatus:  http.StatusConflict,
			GRPC:        codes.Aborted,
			ProblemCode: CodeSessionRefreshConflict,
			Detail:      staticDetail(DetailSessionRefreshConflict),
		},
		{
			Name:        "SessionAuthFailed",
			Sample:      fmt.Errorf("%w", apierrors.ErrSessionAuthFailed),
			Match:       func(e error) bool { return errors.Is(e, apierrors.ErrSessionAuthFailed) },
			HTTPStatus:  http.StatusUnauthorized,
			GRPC:        codes.Unauthenticated,
			ProblemCode: CodeSessionAuthFailed,
			Detail:      staticDetail(DetailSessionAuthFailed),
		},
	}
}

// ContractPipelineRules maps sync/export pipeline errors (subset of domain + default).
func ContractPipelineRules() []Rule {
	return []Rule{
		{
			Name:        "ContractSyncerRejected",
			Sample:      fmt.Errorf("%w: upstream", apierrors.ErrContractSyncerRejected),
			Match:       func(e error) bool { return errors.Is(e, apierrors.ErrContractSyncerRejected) },
			HTTPStatus:  http.StatusBadRequest,
			GRPC:        codes.InvalidArgument,
			ProblemCode: CodeContractSyncerRejected,
			Detail:      func(e error) string { return DetailContractSyncRejected(e) },
		},
		{
			Name:        "StoreAccess",
			Sample:      fmt.Errorf("%w: etcd", apierrors.ErrStoreAccess),
			Match:       func(e error) bool { return errors.Is(e, apierrors.ErrStoreAccess) },
			HTTPStatus:  http.StatusServiceUnavailable,
			GRPC:        codes.Unavailable,
			ProblemCode: CodeStoreUnavailable,
			Detail:      staticDetail(DetailStoreUnavailable),
		},
		{
			Name:        "ContractSyncerUnavailable",
			Sample:      fmt.Errorf("%w: dial", apierrors.ErrContractSyncerUnavailable),
			Match:       func(e error) bool { return errors.Is(e, apierrors.ErrContractSyncerUnavailable) },
			HTTPStatus:  http.StatusBadGateway,
			GRPC:        codes.Unavailable,
			ProblemCode: CodeContractSyncerUnavailable,
			Detail:      staticDetail(DetailContractSyncerUnavailable),
		},
	}
}

// ResolveDomainProblem returns HTTP status, Problem code, and detail for a domain error.
// If no rule matches, returns 500 INTERNAL_ERROR with a generic detail (no err.Error in body).
func ResolveDomainProblem(err error) (httpStatus int, problemCode, detail string) {
	rules := DomainRules()
	for i := range rules {
		r := rules[i]
		if r.Match(err) {
			return r.HTTPStatus, r.ProblemCode, r.Detail(err)
		}
	}
	return http.StatusInternalServerError, CodeInternalError, DetailInternalError
}

// ResolveContractPipelineProblem returns HTTP status, Problem code, and detail for sync/export pipeline.
// Default: 502 CONTRACT_SYNC_PIPELINE_FAILED.
func ResolveContractPipelineProblem(err error) (httpStatus int, problemCode, detail string) {
	rules := ContractPipelineRules()
	for i := range rules {
		r := rules[i]
		if r.Match(err) {
			return r.HTTPStatus, r.ProblemCode, r.Detail(err)
		}
	}
	return http.StatusBadGateway, CodeContractSyncPipelineFailed, DetailContractSyncPipelineFailed
}

// GRPCStatus returns the gRPC code and message for err.
// Mapped errors use the same stable detail as HTTP Problem; unmapped errors use codes.Internal and err.Error().
func GRPCStatus(err error) (codes.Code, string) {
	if err == nil {
		return codes.OK, ""
	}
	rules := DomainRules()
	for i := range rules {
		r := rules[i]
		if r.Match(err) {
			return r.GRPC, r.Detail(err)
		}
	}
	return codes.Internal, err.Error()
}
