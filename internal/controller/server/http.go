package server

import (
	"log"
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

	log.Printf("HTTP server starting on :8080")
	return server.ListenAndServe()
}
