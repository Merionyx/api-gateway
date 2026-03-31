package server

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"

	"merionyx/api-gateway/internal/api-server/container"
)

func StartHTTPServer(c *container.Container) error {
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

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${method} ${path} ${latency}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
	}))

	// Setup routes
	setupRoutes(app, c)

	// Start server
	port := c.Config.Server.HTTPPort
	log.Printf("HTTP Server starting on port %s", port)
	return app.Listen(":" + port)
}

func setupRoutes(app *fiber.App, c *container.Container) {
	// Health check
	app.Get("/health", func(ctx fiber.Ctx) error {
		return ctx.JSON(fiber.Map{
			"status": "ok",
		})
	})

	// JWKS endpoint (well-known)
	app.Get("/.well-known/jwks.json", c.JWTHandler.GetJWKS)

	// API v1
	api := app.Group("/api/v1")

	// JWT Tokens
	api.Post("/tokens", c.JWTHandler.GenerateToken)

	// Signing Keys (для администрирования)
	api.Get("/keys", c.JWTHandler.GetSigningKeys)
}
