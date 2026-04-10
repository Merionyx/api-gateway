package server

import (
	"context"
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"github.com/merionyx/api-gateway/internal/api-server/container"
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
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
	}))

	app.Use(apimetrics.HTTPMiddleware(c.Config.MetricsHTTP.Enabled))
	setupRoutes(app, c)

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

func setupRoutes(app *fiber.App, c *container.Container) {
	app.Get("/health", func(ctx fiber.Ctx) error {
		return ctx.JSON(fiber.Map{
			"status": "ok",
		})
	})

	app.Get("/.well-known/jwks.json", c.JWTHandler.GetJWKS)

	api := app.Group("/api/v1")
	api.Post("/tokens", c.JWTHandler.GenerateToken)
	api.Get("/keys", c.JWTHandler.GetSigningKeys)
	api.Post("/contracts/export", c.ContractsExportHandler.Export)
}
