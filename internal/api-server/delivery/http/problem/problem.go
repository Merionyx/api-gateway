// Package problem maps domain errors to RFC 7807/9457 Problem JSON for HTTP responses.
package problem

import (
	"log/slog"
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	apimetrics "github.com/merionyx/api-gateway/internal/api-server/metrics"
)

// ContentType is the media type for Problem Details (RFC 7807).
const ContentType = "application/problem+json"

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// WithCode builds a Problem with stable `code` and `type` URI (for i18n keys and documentation).
// Pass empty `title` to use the default title for `httpStatus`.
func WithCode(httpStatus int, code, title, detail string) apiserver.Problem {
	if title == "" {
		title = defaultHTTPtitle(httpStatus)
	}
	tu := TypeURI(code)
	return apiserver.Problem{
		Type:   strPtr(tu),
		Code:   strPtr(code),
		Title:  title,
		Status: httpStatus,
		Detail: strPtr(detail),
	}
}

func defaultHTTPtitle(st int) string {
	switch st {
	case http.StatusBadRequest:
		return "Bad Request"
	case http.StatusNotFound:
		return "Not Found"
	case http.StatusBadGateway:
		return "Bad Gateway"
	case http.StatusServiceUnavailable:
		return "Service Unavailable"
	case http.StatusInternalServerError:
		return "Internal Server Error"
	default:
		return "Error"
	}
}

// BadRequest builds a 400 Problem with stable code.
func BadRequest(code, title, detail string) apiserver.Problem {
	return WithCode(http.StatusBadRequest, code, title, detail)
}

// NotFound builds a 404 Problem with stable code.
func NotFound(code, title, detail string) apiserver.Problem {
	return WithCode(http.StatusNotFound, code, title, detail)
}

// InternalError builds a 500 Problem with stable code.
func InternalError(code, title, detail string) apiserver.Problem {
	return WithCode(http.StatusInternalServerError, code, title, detail)
}

// BadGateway builds a 502 Problem with stable code.
func BadGateway(code, title, detail string) apiserver.Problem {
	return WithCode(http.StatusBadGateway, code, title, detail)
}

// ServiceUnavailable builds a 503 Problem with stable code.
func ServiceUnavailable(code, title, detail string) apiserver.Problem {
	return WithCode(http.StatusServiceUnavailable, code, title, detail)
}

// Write sends a Problem response with Content-Type application/problem+json.
func Write(c fiber.Ctx, httpStatus int, p apiserver.Problem) error {
	c.Response().Header.Set("Content-Type", ContentType)
	return c.Status(httpStatus).JSON(p)
}

// RespondError maps a domain error to status + Problem, logs the underlying error, and writes the response.
func RespondError(c fiber.Ctx, err error) error {
	apimetrics.RecordDomainOutcome(apimetrics.MetricsEnabledFromCtx(c), apimetrics.TransportHTTP, err)
	st, p := FromDomain(err)
	logProblemResponse(st, &p, err)
	return Write(c, st, p)
}

func logProblemResponse(st int, p *apiserver.Problem, err error) {
	code := ""
	if p != nil && p.Code != nil {
		code = *p.Code
	}
	slog.Error("http problem response", "status", st, "code", code, "err", err)
}

// WriteInternal writes 500 with a stable code; logs the full error server-side only.
func WriteInternal(c fiber.Ctx, err error) error {
	slog.Error("http internal error", "err", err)
	p := WithCode(http.StatusInternalServerError, CodeInternalError, "", DetailInternalError)
	return Write(c, http.StatusInternalServerError, p)
}

// WriteContractSync maps a Contract Syncer / etcd pipeline error to a Problem, logs the underlying error, and writes the response.
func WriteContractSync(c fiber.Ctx, err error) error {
	apimetrics.RecordContractPipelineOutcome(apimetrics.MetricsEnabledFromCtx(c), err)
	st, p := FromContractSyncPipeline(err)
	logProblemResponse(st, &p, err)
	return Write(c, st, p)
}
