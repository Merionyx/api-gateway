package server

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	oapimw "github.com/oapi-codegen/fiber-middleware/v2"

	"github.com/merionyx/api-gateway/internal/api-server/config"
	"github.com/merionyx/api-gateway/internal/api-server/container"
	httpxmw "github.com/merionyx/api-gateway/internal/api-server/delivery/http/middleware"
	"github.com/merionyx/api-gateway/internal/api-server/gen/apiserver"
	apimetrics "github.com/merionyx/api-gateway/internal/api-server/metrics"
)

// RunHTTPServer runs Fiber until ctx is cancelled, then Shutdown().
func RunHTTPServer(ctx context.Context, c *container.Container) error {
	app := fiber.New(fiber.Config{
		AppName: "API Gateway - API Server",
		ErrorHandler: func(ctx fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return ctx.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} ${latency}\n",
	}))
	installCORSMiddleware(app, c.Config)

	app.Use(apimetrics.HTTPMiddleware(c.Config.MetricsHTTP.Enabled))
	if err := setupRoutes(app, c); err != nil {
		return err
	}

	port := ":" + c.Config.Server.HTTPPort
	slog.Info("HTTP server starting", "port", c.Config.Server.HTTPPort)

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Listen(port)
	}()

	select {
	case <-ctx.Done():
		if err := app.Shutdown(); err != nil {
			slog.Warn("fiber shutdown", "error", err)
		}
		return <-errCh
	case err := <-errCh:
		return err
	}
}

func installCORSMiddleware(app *fiber.App, cfg *config.Config) {
	cc := cfg.Server.CORS
	origins := config.NormalizeCORSAllowOrigins(cc.AllowOrigins)
	wildcard := cc.InsecureAllowWildcard && len(origins) == 1 && origins[0] == "*"

	corsBase := cors.Config{
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Accept", "Authorization", "X-API-Key",
			"Traceparent", "Tracestate", // W3C TraceContext (CORS to browser/edge)
		},
	}

	if wildcard {
		corsBase.AllowOrigins = []string{"*"}
		app.Use(cors.New(corsBase))
		return
	}
	if len(origins) == 0 {
		slog.Info("http: CORS disabled (server.cors.allow_origins empty); set explicit Origins for browser OIDC/SPA (roadmap ш. 24)")
		return
	}
	corsBase.AllowOrigins = origins
	app.Use(cors.New(corsBase))
}

func setupRoutes(app *fiber.App, c *container.Container) error {
	swagger, err := apiserver.GetSwagger()
	if err != nil {
		return fmt.Errorf("embedded OpenAPI spec: %w", err)
	}
	// Skip matching request Host/scheme to spec servers (same pattern as oapi-codegen petstore examples).
	swagger.Servers = nil

	app.Use(httpTraceMiddleware())
	app.Use(oapimw.OapiRequestValidator(swagger))
	app.Use(httpxmw.APISecurity(c.JWTUseCase, c.APIKeyRepository))
	apiserver.RegisterHandlers(app, NewOpenAPIServer(c))
	return nil
}
