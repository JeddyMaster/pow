.PHONY: help build build-server build-client run-server run-client test docker-build docker-up docker-down clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: build-server build-client ## Build both server and client binaries

build-server: ## Build server binary
	@echo "Building server..."
	@go build -o bin/server ./cmd/server
	@echo "Server built successfully: bin/server"

build-client: ## Build client binary
	@echo "Building client..."
	@go build -o bin/client ./cmd/client
	@echo "Client built successfully: bin/client"

run-server: build-server ## Run server locally
	@echo "Starting server..."
	@./bin/server

run-client: build-client ## Run client locally
	@echo "Starting client..."
	@./bin/client

test: ## Run all tests
	@echo "Running tests..."
	@go test -v -race ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./internal/pow/

docker-build: ## Build Docker images
	@echo "Building Docker images..."
	@docker build -f Dockerfile.server -t pow-server .
	@docker build -f Dockerfile.client -t pow-client .
	@echo "Docker images built successfully"

docker-up: ## Start services with Docker Compose
	@echo "Starting services..."
	@docker-compose up --build

docker-down: ## Stop Docker Compose services
	@echo "Stopping services..."
	@docker-compose down

docker-logs: ## Show Docker Compose logs
	@docker-compose logs -f

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f server client
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

lint: ## Run linter
	@echo "Running linter..."
	@golangci-lint run || true

tidy: ## Tidy go modules
	@echo "Tidying go modules..."
	@go mod tidy
