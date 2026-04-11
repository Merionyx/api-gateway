// Package problem maps domain errors to RFC 7807/9457 Problem JSON for HTTP responses.
package problem

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
)

// ContentType is the media type for Problem Details (RFC 7807).
const ContentType = "application/problem+json"

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// BadRequest builds a 400 Problem.
func BadRequest(title, detail string) apiserver.Problem {
	if title == "" {
		title = "Bad Request"
	}
	return apiserver.Problem{
		Title:  title,
		Status: http.StatusBadRequest,
		Detail: strPtr(detail),
	}
}

// NotFound builds a 404 Problem.
func NotFound(title, detail string) apiserver.Problem {
	if title == "" {
		title = "Not Found"
	}
	return apiserver.Problem{
		Title:  title,
		Status: http.StatusNotFound,
		Detail: strPtr(detail),
	}
}

// Internal builds a 500 Problem.
func Internal(detail string) apiserver.Problem {
	return apiserver.Problem{
		Title:  "Internal Server Error",
		Status: http.StatusInternalServerError,
		Detail: strPtr(detail),
	}
}

// BadGateway builds a 502 Problem (upstream / Contract Syncer unavailable).
func BadGateway(detail string) apiserver.Problem {
	return apiserver.Problem{
		Title:  "Bad Gateway",
		Status: http.StatusBadGateway,
		Detail: strPtr(detail),
	}
}

// ServiceUnavailable builds a 503 Problem (e.g. etcd / dependencies down).
func ServiceUnavailable(detail string) apiserver.Problem {
	return apiserver.Problem{
		Title:  "Service Unavailable",
		Status: http.StatusServiceUnavailable,
		Detail: strPtr(detail),
	}
}

// Write sends a Problem response with Content-Type application/problem+json.
func Write(c fiber.Ctx, httpStatus int, p apiserver.Problem) error {
	c.Response().Header.Set("Content-Type", ContentType)
	return c.Status(httpStatus).JSON(p)
}

// RespondError maps a domain error to status + Problem and writes the response.
func RespondError(c fiber.Ctx, err error) error {
	st, p := FromDomain(err)
	return Write(c, st, p)
}

// WriteInternal writes 500 Internal Server Error with err.Error() as detail.
func WriteInternal(c fiber.Ctx, err error) error {
	return Write(c, http.StatusInternalServerError, Internal(err.Error()))
}
