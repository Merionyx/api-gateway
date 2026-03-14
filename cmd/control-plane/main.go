package main

import (
	"log"
	controlplane "merionyx/api-gateway/control-plane/internal/app/control-plane"
)

func main() {
	if err := controlplane.Run(); err != nil {
		log.Fatalf("Failed to run control plane: %v", err)
	}
}
