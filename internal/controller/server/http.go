package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/merionyx/api-gateway/internal/controller/container"
	httpdelivery "github.com/merionyx/api-gateway/internal/controller/delivery/http"
	"github.com/merionyx/api-gateway/internal/shared/serviceapp"
)

// RunHTTPServer serves the probe mux until ctx is cancelled.
func RunHTTPServer(ctx context.Context, container *container.Container) error {
	addr := net.JoinHostPort(container.Config.Server.Host, container.Config.Server.HTTP1Port)
	handler := httpdelivery.NewMux()

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("HTTP probe server starting", "addr", addr)
	if err := serviceapp.RunHTTPServerUntil(ctx, srv, addr); err != nil {
		return fmt.Errorf("http listen %s: %w", addr, err)
	}
	return nil
}
