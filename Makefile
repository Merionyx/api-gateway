.PHONY: run build build-agwctl test clean certs deps start lint fmt docker-build docker-run test-coverage test-coverage-ci help docker-up-dev-ha docker-down-dev-ha test-integration openapi-contract proto-generate proto-install proto-lint proto-breaking

# Variables
BINARY_NAME=universal-server
BUILD_DIR=./bin
DOCKER_REPO=merionyx
DOCKER_IMAGE=merionyx-universal-server
DOCKER_TAG=latest
# Release Dockerfiles: runtime-alpine (shell, wget healthcheck) | runtime-distroless (production)
DOCKER_BUILD_TARGET?=runtime-alpine

# Main commands
run: ## Run the server
	go run cmd/controller/main.go

build: ## Build binary
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $(BUILD_DIR)/$(BINARY_NAME) cmd/controller/main.go

build-cli: ## Build agwctl CLI (embeds git tag/rev/time via -ldflags when git available)
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/agwctl \
		-ldflags "-s -w \
		-X 'github.com/merionyx/api-gateway/internal/cli/version.Version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)' \
		-X 'github.com/merionyx/api-gateway/internal/cli/version.Commit=$(shell git rev-parse --short HEAD 2>/dev/null)' \
		-X 'github.com/merionyx/api-gateway/internal/cli/version.BuildTime=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)'" \
		./cmd/agwctl

test: ## Run all tests
	go test -v ./...

test-coverage: ## Run tests with coverage
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-coverage-ci: ## Unit tests + coverage gate (see .coverage-min; without merionyx/api-gateway/pkg/*)
	bash scripts/ci/check-coverage.sh

test-integration: ## Run integration tests (starts etcd in Docker via scripts/dev/run-integration-tests.sh)
	bash scripts/dev/run-integration-tests.sh

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
docker-build: ## Build Docker images (default target: Alpine; set DOCKER_BUILD_TARGET=runtime-distroless for distroless)
	@DOCKER_BUILDKIT=1 docker build --target $(DOCKER_BUILD_TARGET) --build-arg SERVICE=controller      --build-arg OCI_NAME=Controller        -t $(DOCKER_REPO)/api-gateway-controller:$(DOCKER_TAG) -f build/release/Dockerfile .
	@DOCKER_BUILDKIT=1 docker build --target $(DOCKER_BUILD_TARGET) --build-arg SERVICE=api-server      --build-arg OCI_NAME='API Server'      -t $(DOCKER_REPO)/api-gateway-api-server:$(DOCKER_TAG) -f build/release/Dockerfile .
	@DOCKER_BUILDKIT=1 docker build --target $(DOCKER_BUILD_TARGET) --build-arg SERVICE=contract-syncer --build-arg OCI_NAME='Contract Syncer' -t $(DOCKER_REPO)/api-gateway-contract-syncer:$(DOCKER_TAG) -f build/release/Dockerfile .
	@DOCKER_BUILDKIT=1 docker build --target $(DOCKER_BUILD_TARGET) --build-arg SERVICE=auth-sidecar    --build-arg OCI_NAME='Auth Sidecar'    -t $(DOCKER_REPO)/api-gateway-auth-sidecar:$(DOCKER_TAG) -f build/release/Dockerfile .
	@DOCKER_BUILDKIT=1 docker build --target $(DOCKER_BUILD_TARGET) --build-arg SERVICE=mock-service    --build-arg OCI_NAME='Mock Service'    -t $(DOCKER_REPO)/api-gateway-mock-service:$(DOCKER_TAG) -f build/release/Dockerfile .

docker-push: ## Push Docker image
	@docker push $(DOCKER_REPO)/api-gateway-controller:$(DOCKER_TAG)
	@docker push $(DOCKER_REPO)/api-gateway-api-server:$(DOCKER_TAG)
	@docker push $(DOCKER_REPO)/api-gateway-contract-syncer:$(DOCKER_TAG)
	@docker push $(DOCKER_REPO)/api-gateway-auth-sidecar:$(DOCKER_TAG)
	@docker push $(DOCKER_REPO)/api-gateway-mock-service:$(DOCKER_TAG)

docker-run: ## Run Docker container
	docker run -p 8080:8080 -p 8443:8443 -p 8444:8444 $(DOCKER_IMAGE):$(DOCKER_TAG)

docker-up:
	docker-compose \
		-p 'merionyx-api-gateway' \
		-f ./deployments/dev/docker/compose.app.yaml \
		-f ./deployments/dev/docker/compose.sidecar.yaml \
		-f ./deployments/dev/docker/compose.etcd.yaml \
		-f ./deployments/dev/docker/compose.envoy.yaml \
		-f ./deployments/dev/docker/compose.mock-service.yaml \
		up --build

docker-down:
	docker-compose \
		-p 'merionyx-api-gateway' \
		-f ./deployments/dev/docker/compose.app.yaml \
		-f ./deployments/dev/docker/compose.sidecar.yaml \
		-f ./deployments/dev/docker/compose.etcd.yaml \
		-f ./deployments/dev/docker/compose.envoy.yaml \
		-f ./deployments/dev/docker/compose.mock-service.yaml \
		down

docker-up-dev: ## Docker dev stack (single replicas) + watch
	docker-compose \
		-p 'merionyx-api-gateway' \
		-f ./deployments/dev/docker/compose.app.dev.yaml \
		-f ./deployments/dev/docker/compose.sidecar.dev.yaml \
		-f ./deployments/dev/docker/compose.etcd.yaml \
		-f ./deployments/dev/docker/compose.envoy.yaml \
		-f ./deployments/dev/docker/compose.mock-service.yaml \
		up --build --watch

docker-down-dev: ## Stop docker dev stack
	docker-compose \
		-p 'merionyx-api-gateway' \
		-f ./deployments/dev/docker/compose.app.dev.yaml \
		-f ./deployments/dev/docker/compose.sidecar.dev.yaml \
		-f ./deployments/dev/docker/compose.etcd.yaml \
		-f ./deployments/dev/docker/compose.envoy.yaml \
		-f ./deployments/dev/docker/compose.mock-service.yaml \
		down

docker-up-dev-ha: ## HA dev: 3 API Server, 6 controllers, 6 Envoy, HAProxy (project merionyx-api-gateway-ha)
	docker compose \
		-p 'merionyx-api-gateway-ha' \
		-f ./deployments/dev/docker/compose.app.ha.dev.yaml \
		-f ./deployments/dev/docker/compose.sidecar.dev.ha.yaml \
		-f ./deployments/dev/docker/compose.etcd.yaml \
		-f ./deployments/dev/docker/compose.envoy.ha.dev.yaml \
		-f ./deployments/dev/docker/compose.mock-service.yaml \
		up --build

docker-down-dev-ha: ## Stop HA dev stack
	docker compose \
		-p 'merionyx-api-gateway-ha' \
		-f ./deployments/dev/docker/compose.app.ha.dev.yaml \
		-f ./deployments/dev/docker/compose.sidecar.dev.ha.yaml \
		-f ./deployments/dev/docker/compose.etcd.yaml \
		-f ./deployments/dev/docker/compose.envoy.ha.dev.yaml \
		-f ./deployments/dev/docker/compose.mock-service.yaml \
		down

dev: ## Development mode with hot reload
	air -c .air.toml

PROTO_MODULE ?= github.com/merionyx/api-gateway
PROTO_ROOT ?= apis/proto
# Sources: apis/proto/merionyx/gateway/<domain>/v1 — package merionyx.gateway.<domain>.v1 (Buf PACKAGE_DIRECTORY_MATCH).
PROTO_FILES := $(shell find $(PROTO_ROOT)/merionyx/gateway -type f -name '*.proto' 2>/dev/null | LC_ALL=C sort)


proto-generate: ## Generate protobuf code (writes under pkg/grpc/...; see PROTO_MODULE)
	protoc -I $(PROTO_ROOT) \
		--go_out=. --go_opt=module=$(PROTO_MODULE) --go_opt=paths=import \
		--go-grpc_out=. --go-grpc_opt=module=$(PROTO_MODULE) --go-grpc_opt=paths=import \
		$(PROTO_FILES)

# Call make from the root repository (as usual). cd is not needed: buf gets the module path as an argument.
proto-lint: ## Lint .proto (requires buf: https://buf.build/docs/installation)
	buf lint $(PROTO_ROOT)

proto-breaking: ## Detect breaking API changes vs main (requires buf + git; cwd = repo root)
	buf breaking $(PROTO_ROOT) --against ".git#branch=main"

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
	@echo "subjectAltName=DNS:etcd-api-server-1,DNS:etcd-api-server-2,DNS:etcd-api-server-3,DNS:etcd-controller-dev-1,DNS:etcd-controller-dev-2,DNS:etcd-controller-dev-3,DNS:etcd-controller-prod-1,DNS:etcd-controller-prod-2,DNS:etcd-controller-prod-3,DNS:localhost,IP:127.0.0.1" > secrets/certs/etcd/san.cnf
	
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

LOCALBIN ?= $(shell pwd)/bin
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
CONTROLLER_TOOLS_VERSION ?= v0.18.0

.PHONY: controller-gen-bin
controller-gen-bin: $(CONTROLLER_GEN)
$(CONTROLLER_GEN):
	@mkdir -p $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: apis-generate
generate-crds: controller-gen-bin ## DeepCopy + CRD YAML into ../dist/crds
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./pkg/apis/crd/v1alpha1/..."
	@mkdir -p ./dist/crds
	$(CONTROLLER_GEN) crd paths="./pkg/apis/crd/v1alpha1/..." output:crd:artifacts:config=./dist/crds

# Help
help: ## Show help
	@echo "Available commands:"
	@echo ""
	@awk 'BEGIN {FS = ":.*?## "; printf "%-20s %s\n", "Command", "Description"; printf "%-20s %s\n", "-------", "-----------"} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Default target shows help
.DEFAULT_GOAL := help