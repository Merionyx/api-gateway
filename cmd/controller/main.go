package main

import (
	"log"
	controller "merionyx/api-gateway/internal/controller/app"
)

func main() {
	if err := controller.Run(); err != nil {
		log.Fatalf("Failed to run control plane: %v", err)
	}
}
