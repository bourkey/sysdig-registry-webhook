# Makefile for Registry Webhook Scanner

# Variables
IMAGE_NAME ?= registry-webhook-scanner
IMAGE_TAG ?= latest
REGISTRY ?= docker.io
FULL_IMAGE := $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

# Go variables
GO_VERSION := 1.21
BINARY_NAME := scanner-webhook
MAIN_PATH := ./cmd/scanner-webhook

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the Go binary locally
	@echo "Building $(BINARY_NAME)..."
	CGO_ENABLED=0 go build -o $(BINARY_NAME) $(MAIN_PATH)

.PHONY: test
test: ## Run unit tests
	@echo "Running tests..."
	go test -v -race -cover ./...

.PHONY: lint
lint: ## Run linters
	@echo "Running golangci-lint..."
	golangci-lint run ./...

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "Building Docker image: $(FULL_IMAGE)"
	docker build -t $(FULL_IMAGE) .
	@echo "Image built successfully: $(FULL_IMAGE)"

.PHONY: docker-test
docker-test: docker-build ## Test Docker image locally
	@echo "Testing Docker image..."
	@echo "Starting container..."
	docker run -d --name scanner-test \
		-p 8080:8080 \
		-e SYSDIG_API_TOKEN=test-token \
		$(FULL_IMAGE) || true
	@sleep 3
	@echo "Testing health endpoint..."
	@curl -f http://localhost:8080/health || (docker logs scanner-test && exit 1)
	@echo "Testing readiness endpoint..."
	@curl -f http://localhost:8080/ready || (docker logs scanner-test && exit 1)
	@echo "Cleaning up..."
	@docker stop scanner-test
	@docker rm scanner-test
	@echo "Docker image test passed!"

.PHONY: docker-run
docker-run: docker-build ## Run Docker image locally with example config
	@echo "Running Docker image locally..."
	docker run --rm -it \
		-p 8080:8080 \
		-v $(PWD)/config.example.yaml:/app/config/config.yaml:ro \
		-e CONFIG_FILE=/app/config/config.yaml \
		-e SYSDIG_API_TOKEN=${SYSDIG_API_TOKEN} \
		-e LOG_LEVEL=debug \
		$(FULL_IMAGE)

.PHONY: docker-push
docker-push: docker-build ## Push Docker image to registry
	@echo "Pushing $(FULL_IMAGE) to registry..."
	docker push $(FULL_IMAGE)

.PHONY: docker-scan
docker-scan: docker-build ## Scan Docker image for vulnerabilities
	@echo "Scanning Docker image for vulnerabilities..."
	docker scout quickview $(FULL_IMAGE) || true
	docker scout cves $(FULL_IMAGE) || true

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME)
	go clean -cache

.PHONY: deps
deps: ## Download Go dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

.PHONY: fmt
fmt: ## Format Go code
	@echo "Formatting code..."
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

.DEFAULT_GOAL := help
