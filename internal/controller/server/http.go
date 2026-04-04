package server

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"merionyx/api-gateway/internal/controller/container"
	"merionyx/api-gateway/internal/controller/delivery/http"
)

func StartHTTPServer(container *container.Container) error {
	addr := net.JoinHostPort(container.Config.Server.Host, container.Config.Server.HTTP1Port)
	handler := httpdelivery.NewMux(container.Config)

	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	slog.Info("HTTP probe server starting", "addr", addr)
	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("http listen %s: %w", addr, err)
	}
	return nil
}
