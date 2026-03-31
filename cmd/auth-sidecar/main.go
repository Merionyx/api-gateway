package main

import (
	"log"

	"merionyx/api-gateway/internal/auth-sidecar/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatalf("Failed to run auth sidecar: %v", err)
	}
}
