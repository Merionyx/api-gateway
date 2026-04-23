package main

import (
	"log/slog"
	"os"

	"github.com/merionyx/api-gateway/internal/sidecar/app"
)

func main() {
	if err := app.Run(); err != nil {
		slog.Error("failed to run sidecar", "error", err)
		os.Exit(1)
	}
}
