package server

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

// httpTraceMiddleware runs W3C TraceContext extraction, starts a request span, stores the
// trace on [fiber.Ctx] (handlers use [fiber.Ctx.Context] in use case calls), and on exit
// records HTTP status. Span names are "{METHOD} {path}" (route template from [fiber.Ctx.FullPath]
// after a match) — not `telemetry.SpanName` with a package prefix, so names don’t look
// like `.../server.GET` glued to the path.
func httpTraceMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
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
