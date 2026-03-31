package main

import (
	"log"
	controller "merionyx/api-gateway/internal/api-server/app"
)

func main() {
	if err := controller.Run(); err != nil {
		log.Fatalf("Failed to run control plane: %v", err)
	}
}
