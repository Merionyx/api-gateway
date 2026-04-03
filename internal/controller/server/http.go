package server

import (
	"log/slog"
	"merionyx/api-gateway/internal/controller/container"
	"net/http"
)

func StartHTTPServer(container *container.Container) error {
	// Setup routes
	handler := container.Router.SetupRoutes()

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	slog.Info("HTTP server starting", "addr", ":8080")
	return server.ListenAndServe()
}
