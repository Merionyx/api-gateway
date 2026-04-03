package main

import (
	"log/slog"
	"os"

	"merionyx/api-gateway/internal/auth-sidecar/app"
)

func main() {
	if err := app.Run(); err != nil {
		slog.Error("failed to run auth sidecar", "error", err)
		os.Exit(1)
	}
}
