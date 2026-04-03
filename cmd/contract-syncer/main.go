package main

import (
	"log/slog"
	"os"

	syncer "merionyx/api-gateway/internal/contract-syncer/app"
)

func main() {
	if err := syncer.Run(); err != nil {
		slog.Error("failed to run contract syncer", "error", err)
		os.Exit(1)
	}
}
