package openapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/merionyx/api-gateway/internal/api-server/delivery/http/problem"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
)

func jsonETag(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return fmt.Sprintf(`"%x"`, sum), nil
}

func ifNoneMatchMatches(ifNoneMatch string, etag string) bool {
	in := strings.TrimSpace(ifNoneMatch)
	in = strings.TrimPrefix(in, "W/")
	in = strings.Trim(in, `"`)
	want := strings.Trim(etag, `"`)
	return in == want
}

func internalProblem() apiserver.Problem {
	return problem.WithCode(http.StatusInternalServerError, problem.CodeInternalError, "", problem.DetailInternalError)
}

func asInternalProblemResponse() apiserver.InternalErrorApplicationProblemPlusJSONResponse {
	return apiserver.InternalErrorApplicationProblemPlusJSONResponse(internalProblem())
}

func mapDomainProblem(err error, allowed ...int) (int, apiserver.Problem) {
	st, p := problem.FromDomain(err)
	for i := range allowed {
		if st == allowed[i] {
			return st, p
		}
	}
	return http.StatusInternalServerError, internalProblem()
}

func mapContractPipelineProblem(err error, allowed ...int) (int, apiserver.Problem) {
	st, p := problem.FromContractSyncPipeline(err)
	for i := range allowed {
		if st == allowed[i] {
			return st, p
		}
	}
	return http.StatusInternalServerError, internalProblem()
}

func durationFromOptionalFormSeconds(v *int) time.Duration {
	if v == nil {
		return 0
	}
	if *v <= 0 {
		return 0
	}
	return time.Duration(*v) * time.Second
}

func usesBasicAuthorizationHeader(h string) bool {
	raw := strings.TrimSpace(h)
	if raw == "" {
		return false
	}
	if strings.EqualFold(raw, "Basic") {
		return true
	}
	if len(raw) < len("Basic ")+1 {
		return false
	}
	return strings.EqualFold(raw[:len("Basic ")], "Basic ")
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
