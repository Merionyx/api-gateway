package openapi

import (
	"context"
	"fmt"

	"github.com/gofiber/fiber/v3"
)

type strictFiberCtxKey struct{}

func BindFiberContextForStrictHandlers() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.SetContext(context.WithValue(c.Context(), strictFiberCtxKey{}, c))
		return c.Next()
	}
}

func fiberCtxFromStrictContext(ctx context.Context) (fiber.Ctx, error) {
	if ctx == nil {
		return nil, fmt.Errorf("missing strict request context")
	}
	fc, ok := ctx.Value(strictFiberCtxKey{}).(fiber.Ctx)
	if !ok || fc == nil {
		return nil, fmt.Errorf("missing fiber context in strict request context")
	}
	return fc, nil
}
