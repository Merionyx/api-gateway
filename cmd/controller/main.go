package main

import (
	"log/slog"
	"os"

	controller "merionyx/api-gateway/internal/controller/app"
)

func main() {
	if err := controller.Run(); err != nil {
		slog.Error("failed to run controller", "error", err)
		os.Exit(1)
	}
}
