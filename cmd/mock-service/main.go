package main

import (
	"log/slog"
	"os"

	"merionyx/api-gateway/internal/mock-service/app"
)

func main() {
	if err := app.Run(); err != nil {
		slog.Error("mock service failed", "error", err)
		os.Exit(1)
	}
}
