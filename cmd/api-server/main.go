package main

import (
	"log/slog"
	"os"

	apiserver "github.com/merionyx/api-gateway/internal/api-server/app"
)

func main() {
	if err := apiserver.Run(); err != nil {
		slog.Error("failed to run API server", "error", err)
		os.Exit(1)
	}
}
