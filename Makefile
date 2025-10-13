.PHONY: run build test clean certs deps start lint fmt docker-build docker-run test-coverage help

# Variables
BINARY_NAME=universal-server
BUILD_DIR=./bin
DOCKER_IMAGE=merionyx-universal-server
DOCKER_TAG=latest

# Main commands
run: ## Run the server
	go run cmd/control-plane/main.go

build: ## Build binary
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $(BUILD_DIR)/$(BINARY_NAME) cmd/control-plane/main.go

test: ## Run all tests
	go test -v ./...

test-coverage: ## Run tests with coverage
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-integration: ## Run integration tests
	go test -v -tags=integration ./test/...

clean: ## Clean build artifacts
	rm -rf $(BUILD_DIR)
	rm -rf certs/
	rm -f coverage.out coverage.html

deps: ## Install and update dependencies
	go mod download
	go mod tidy
	go mod verify

fmt: ## Format code
	go fmt ./...
	goimports -w .

lint: ## Lint code
	golangci-lint run ./...

# Docker commands
docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-run: ## Run Docker container
	docker run -p 8080:8080 -p 8443:8443 -p 8444:8444 $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-up:
	docker-compose \
		-p 'merionyx-api-gateway-control-plane' \
		-f ./deployments/docker/compose.postgresql.yaml \
		-f ./deployments/docker/compose.app.yaml \
		up --build --watch

docker-down:
	docker-compose \
		-p 'merionyx-api-gateway-control-plane' \
		-f ./deployments/docker/compose.postgresql.yaml \
		-f ./deployments/docker/compose.app.yaml \
		down

dev: ## Development mode with hot reload
	air -c .air.toml

pg-dump: ## Dump PostgreSQL schema
	pg_dump --schema-only "postgresql://postgres:postgres@localhost:5432/postgres" > ./databases/postgres/service/schema.sql

sqlc-generate: ## Generate SQLC code
	sqlc generate

sqlc-update: pg-dump sqlc-generate ## Update SQLC code

# Help
help: ## Show help
	@echo "Available commands:"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "; printf "%-20s %s\n", "Command", "Description"; printf "%-20s %s\n", "-------", "-----------"} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Default target shows help
.DEFAULT_GOAL := help