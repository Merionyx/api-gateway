package main

import (
	"log/slog"
	"os"

	"github.com/merionyx/api-gateway/internal/controller/app"
)

func main() {
	if err := app.Run(); err != nil {
		slog.Error("failed to run controller", "error", err)
		os.Exit(1)
	}
}
