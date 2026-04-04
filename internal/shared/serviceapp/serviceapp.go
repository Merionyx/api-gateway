package serviceapp

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// NewJSONLogger builds the standard JSON slog handler used by all binaries in this module.
func NewJSONLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

// SetDefaultLogger installs logger as the default slog logger.
func SetDefaultLogger(logger *slog.Logger) {
	slog.SetDefault(logger)
}

// WaitSignalOrContext blocks until SIGINT/SIGTERM or ctx is cancelled (whichever comes first).
func WaitSignalOrContext(ctx context.Context) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigChan:
		slog.Info("Shutdown signal received")
	case <-ctx.Done():
		slog.Info("Context cancelled")
	}
}
