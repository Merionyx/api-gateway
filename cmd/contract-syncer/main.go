package main

import (
	"log"
	syncer "merionyx/api-gateway/internal/contract-syncer/app"
)

func main() {
	if err := syncer.Run(); err != nil {
		log.Fatalf("Failed to run contract syncer: %v", err)
	}
}
