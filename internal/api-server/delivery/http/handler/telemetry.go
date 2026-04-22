package handler

import (
	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel/trace"

	"github.com/merionyx/api-gateway/internal/shared/telemetry"
)

// spanHandlerPkg is the import path of this package within the module (child spans under Fiber HTTP root).
const spanHandlerPkg = "internal/api-server/delivery/http/handler"

// beginHandlerSpan starts a child span and attaches the traced context to c. Caller: defer span.End();
// on usecase/transport error returns, use telemetry.MarkError(span, err).
func beginHandlerSpan(c fiber.Ctx, funcName string) trace.Span {
	ctx, span := telemetry.Start(c.Context(), telemetry.SpanName(spanHandlerPkg, funcName))
	c.SetContext(ctx)
	return span
}
