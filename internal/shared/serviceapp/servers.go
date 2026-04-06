package serviceapp

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"google.golang.org/grpc"
)

// DefaultShutdownTimeout is the maximum time for HTTP Server.Shutdown after SIGTERM.
const DefaultShutdownTimeout = 30 * time.Second

// RunHTTPServerUntil serves until ctx is cancelled, then calls Shutdown with DefaultShutdownTimeout.
func RunHTTPServerUntil(ctx context.Context, srv *http.Server, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer func() { _ = ln.Close() }()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
		defer cancel()
		if shutErr := srv.Shutdown(shutCtx); shutErr != nil {
			slog.Warn("http server shutdown", "error", shutErr)
		}
		serveErr := <-errCh
		if errors.Is(serveErr, http.ErrServerClosed) {
			return nil
		}
		return serveErr
	case serveErr := <-errCh:
		if errors.Is(serveErr, http.ErrServerClosed) {
			return nil
		}
		return serveErr
	}
}

// RunGRPCServeUntil serves until ctx is cancelled, then GracefulStop.
func RunGRPCServeUntil(ctx context.Context, srv *grpc.Server, lis net.Listener) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(lis)
	}()

	select {
	case <-ctx.Done():
		srv.GracefulStop()
		err := <-errCh
		return err
	case err := <-errCh:
		return err
	}
}
