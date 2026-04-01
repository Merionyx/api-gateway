.PHONY: run build test clean certs deps start lint fmt docker-build docker-run test-coverage help

# Variables
BINARY_NAME=universal-server
BUILD_DIR=./bin
DOCKER_IMAGE=merionyx-universal-server
DOCKER_TAG=latest

# Main commands
run: ## Run the server
	go run cmd/controller/main.go

build: ## Build binary
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $(BUILD_DIR)/$(BINARY_NAME) cmd/controller/main.go

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
		-p 'merionyx-api-gateway' \
		-f ./deployments/docker/compose.app.dev.yaml \
		-f ./deployments/docker/compose.sidecar.dev.yaml \
		-f ./deployments/docker/compose.etcd.yaml \
		-f ./deployments/docker/compose.envoy.yaml \
		-f ./deployments/docker/compose.mock-service.yaml \
		up --build --watch

docker-down:
	docker-compose \
		-p 'merionyx-api-gateway' \
		-f ./deployments/docker/compose.app.dev.yaml \
		-f ./deployments/docker/compose.sidecar.dev.yaml \
		-f ./deployments/docker/compose.etcd.yaml \
		-f ./deployments/docker/compose.envoy.yaml \
		-f ./deployments/docker/compose.mock-service.yaml \
		down

dev: ## Development mode with hot reload
	air -c .air.toml

proto-generate: ## Generate protobuf code
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/v1/*.proto && \
	cp ./api/proto/v1/snapshots_grpc.pb.go ./pkg/api/snapshots/v1/snapshots_grpc.pb.go && \
	cp ./api/proto/v1/snapshots.pb.go ./pkg/api/snapshots/v1/snapshots.pb.go && \
	cp ./api/proto/v1/schemas_grpc.pb.go ./pkg/api/schemas/v1/schemas_grpc.pb.go && \
	cp ./api/proto/v1/schemas.pb.go ./pkg/api/schemas/v1/schemas.pb.go && \
	cp ./api/proto/v1/environment_grpc.pb.go ./pkg/api/environments/v1/environment_grpc.pb.go && \
	cp ./api/proto/v1/environment.pb.go ./pkg/api/environments/v1/environment.pb.go && \
	cp ./api/proto/v1/auth_grpc.pb.go ./pkg/api/auth/v1/auth_grpc.pb.go && \
	cp ./api/proto/v1/auth.pb.go ./pkg/api/auth/v1/auth.pb.go && \
	rm -rf ./api/proto/v1/*.pb.go

proto-install: ## Install protobuf tools
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

generate-etcd-certs: ## Generate etcd certificates
	@echo "\033[1;34m🔐 Generating etcd certificates...\033[0m"
	@mkdir -p secrets/certs/etcd
	
	@echo "\033[0;36m→ Generating CA private key...\033[0m"
	@openssl ecparam -genkey -name prime256v1 -out secrets/certs/etcd/ca-key.pem
	
	@echo "\033[0;36m→ Generating CA certificate...\033[0m"
	@openssl req -new -x509 -days 3650 -key secrets/certs/etcd/ca-key.pem \
		-out secrets/certs/etcd/ca.pem \
		-subj "/CN=etcd-ca"

	@echo "\033[0;36m→ Generating server private key...\033[0m"
	@openssl ecparam -genkey -name prime256v1 -out secrets/certs/etcd/server-key.pem
	
	@echo "\033[0;36m→ Generating server CSR...\033[0m"
	@openssl req -new -key secrets/certs/etcd/server-key.pem \
		-out secrets/certs/etcd/server.csr \
		-subj "/CN=etcd-server"
	
	@echo "\033[0;36m→ Creating OpenSSL config for SAN...\033[0m"
	@echo "subjectAltName=DNS:etcd,DNS:localhost,IP:127.0.0.1" > secrets/certs/etcd/san.cnf
	
	@echo "\033[0;36m→ Signing server certificate...\033[0m"
	@openssl x509 -req -in secrets/certs/etcd/server.csr \
		-CA secrets/certs/etcd/ca.pem \
		-CAkey secrets/certs/etcd/ca-key.pem \
		-CAcreateserial \
		-out secrets/certs/etcd/server.pem \
		-days 3650 \
		-extfile secrets/certs/etcd/san.cnf
	
	@echo "\033[0;36m→ Generating peer private key...\033[0m"
	@openssl ecparam -genkey -name prime256v1 -out secrets/certs/etcd/peer-key.pem
	
	@echo "\033[0;36m→ Generating peer CSR...\033[0m"
	@openssl req -new -key secrets/certs/etcd/peer-key.pem \
		-out secrets/certs/etcd/peer.csr \
		-subj "/CN=etcd-peer"
	
	@echo "\033[0;36m→ Signing peer certificate...\033[0m"
	@openssl x509 -req -in secrets/certs/etcd/peer.csr \
		-CA secrets/certs/etcd/ca.pem \
		-CAkey secrets/certs/etcd/ca-key.pem \
		-CAcreateserial \
		-out secrets/certs/etcd/peer.pem \
		-days 3650 \
		-extfile secrets/certs/etcd/san.cnf
	
	@echo "\033[0;36m→ Generating client private key...\033[0m"
	@openssl ecparam -genkey -name prime256v1 -out secrets/certs/etcd/client-key.pem
	
	@echo "\033[0;36m→ Generating client CSR...\033[0m"
	@openssl req -new -key secrets/certs/etcd/client-key.pem \
		-out secrets/certs/etcd/client.csr \
		-subj "/CN=etcd-client"
	
	@echo "\033[0;36m→ Signing client certificate...\033[0m"
	@openssl x509 -req -in secrets/certs/etcd/client.csr \
		-CA secrets/certs/etcd/ca.pem \
		-CAkey secrets/certs/etcd/ca-key.pem \
		-CAcreateserial \
		-out secrets/certs/etcd/client.pem \
		-days 3650
	
	@echo "\033[0;33m→ Cleaning up temporary files...\033[0m"
	@rm -f secrets/certs/etcd/*.csr secrets/certs/etcd/*.srl secrets/certs/etcd/san.cnf
	
	@echo "\033[1;32m✓ etcd certificates generated successfully!\033[0m"
	@echo "\033[0;32m  📁 Location: secrets/certs/etcd/\033[0m"
	@echo "\033[0;90m  • CA: ca.pem, ca-key.pem\033[0m"
	@echo "\033[0;90m  • Server: server.pem, server-key.pem\033[0m"
	@echo "\033[0;90m  • Peer: peer.pem, peer-key.pem\033[0m"
	@echo "\033[0;90m  • Client: client.pem, client-key.pem\033[0m"

# JWT Keys Management
.PHONY: jwt-generate-ed25519
generate-ed25519-key:
	@echo "Generating Ed25519 key..."
	@mkdir -p secrets/api-server/keys/jwt
	@openssl genpkey -algorithm ED25519 -out secrets/api-server/keys/jwt/api-server-key-$(shell date +%Y-%m-%d).key
	@chmod 600 secrets/api-server/keys/jwt/api-server-key-$(shell date +%Y-%m-%d).key
	@echo "✓ Generated: secrets/api-server/keys/jwt/api-server-key-$(shell date +%Y-%m-%d).key"

.PHONY: jwt-generate-rsa
generate-rsa-key:
	@echo "Generating RSA 2048 key..."
	@mkdir -p secrets/api-server/keys/jwt
	@openssl genrsa -out secrets/api-server/keys/jwt/api-server-rsa-$(shell date +%Y-%m-%d).key 2048
	@chmod 600 secrets/api-server/keys/jwt/api-server-rsa-$(shell date +%Y-%m-%d).key
	@echo "✓ Generated: secrets/api-server/keys/jwt/api-server-rsa-$(shell date +%Y-%m-%d).key"

# Help
help: ## Show help
	@echo "Available commands:"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "; printf "%-20s %s\n", "Command", "Description"; printf "%-20s %s\n", "-------", "-----------"} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Default target shows help
.DEFAULT_GOAL := help