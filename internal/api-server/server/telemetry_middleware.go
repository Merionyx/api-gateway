package server

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

// path for span names, aligned with the package that wires the HTTP app (one span per request).
const spanAPIHTTPServerPkg = "internal/api-server/server"

// httpTraceMiddleware runs W3C TraceContext extraction, starts a request span, stores the
// trace on [fiber.Ctx] (handlers use [fiber.Ctx.Context] in use case calls), and on exit
// records HTTP status. The span name is updated to the matched route pattern after
// [fiber.Ctx.Next] when available.
func httpTraceMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx := c.Context()
		ctx = telemetry.ExtractIncomingHTTP(ctx, http.Header(c.GetReqHeaders()))
		prov := c.Method() + " " + c.Path()
		ctx, span := telemetry.Start(ctx, telemetry.SpanName(spanAPIHTTPServerPkg, prov))
		c.SetContext(ctx)

		err := c.Next()

		if c.Matched() {
			if fp := c.FullPath(); fp != "" {
				span.SetName(telemetry.SpanName(spanAPIHTTPServerPkg, c.Method()+" "+fp))
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
