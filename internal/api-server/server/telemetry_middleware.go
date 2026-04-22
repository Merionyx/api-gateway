package server

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

// isK8SProbePath reports paths for which we avoid creating a root HTTP span
// (high-frequency; little diagnostic value in traces).
func isK8SProbePath(path string) bool {
	if path == "/health" || path == "/ready" {
		return true
	}
	if path == "" {
		return false
	}
	// e.g. /v1/health, /api/v1/ready
	if len(path) > len("/health") && strings.HasSuffix(path, "/health") {
		return true
	}
	if len(path) > len("/ready") && strings.HasSuffix(path, "/ready") {
		return true
	}
	return false
}

// httpTraceMiddleware runs W3C TraceContext extraction, starts a request span, stores the
// trace on [fiber.Ctx] (handlers use [fiber.Ctx.Context] in use case calls), and on exit
// records HTTP status. Span names are "{METHOD} {path}" (route template from [fiber.Ctx.FullPath]
// after a match) — not `telemetry.SpanName` with a package prefix, so names don’t look
// like `.../server.GET` glued to the path.
func httpTraceMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if p := c.Path(); isK8SProbePath(p) {
			return c.Next()
		}
		ctx := c.Context()
		ctx = telemetry.ExtractIncomingHTTP(ctx, http.Header(c.GetReqHeaders()))
		prov := c.Method() + " " + c.Path()
		ctx, span := telemetry.Start(ctx, prov)
		c.SetContext(ctx)

		err := c.Next()

		if c.Matched() {
			if fp := c.FullPath(); fp != "" {
				span.SetName(c.Method() + " " + fp)
			}
		}

		code := c.Response().StatusCode()
		if code == 0 {
			code = fiber.StatusInternalServerError
		}
		telemetry.SetHTTPStatus(span, code)
		if err != nil {
			telemetry.MarkError(span, err)
		}
		span.End()
		return err
	}
}
