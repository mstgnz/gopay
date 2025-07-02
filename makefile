# GoPay Makefile

.PHONY: help test test-unit test-integration test-coverage build run clean lint format deps dev opensearch-start opensearch-stop opensearch-status logs-query docker-build docker-run docker-stop docker-logs ci-test ci-build integration-help

# Default target
help: ## Show this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Test commands
test: test-unit ## Run all tests (unit tests only by default)

test-unit: ## Run unit tests
	@echo "🧪 Running unit tests..."
	@go test -v ./...

test-integration: ## Run integration tests (requires credentials)
	@echo "🧪 Running integration tests..."
	@if [ -z "$(IYZICO_TEST_API_KEY)" ]; then \
		echo "❌ IYZICO_TEST_API_KEY not set. Integration tests skipped."; \
		echo "Set the following environment variables:"; \
		echo "  export IYZICO_TEST_ENABLED=true"; \
		echo "  export IYZICO_TEST_API_KEY=your_sandbox_api_key"; \
		echo "  export IYZICO_TEST_SECRET_KEY=your_sandbox_secret_key"; \
		exit 1; \
	fi
	@go test -v ./gateway/iyzico/ -run TestIntegration

test-coverage: ## Run tests with coverage report
	@echo "📊 Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-benchmark: ## Run benchmark tests
	@echo "⚡ Running benchmark tests..."
	@go test -bench=. -benchmem ./...

test-iyzico: ## Run all İyzico tests (unit + integration)
	@echo "🏦 Running İyzico tests..."
	@go test -v ./gateway/iyzico/

test-iyzico-integration: ## Run İyzico integration tests only
	@echo "🏦 Running İyzico integration tests..."
	@go test -v ./gateway/iyzico/ -run TestIntegration

# Build commands
build: ## Build the application
	@echo "🔨 Building application..."
	@go build -o bin/gopay ./cmd/main.go

build-docker: ## Build Docker image
	@echo "🐳 Building Docker image..."
	@docker build -t gopay:latest .

# Run commands
run: ## Run the application
	@echo "🚀 Starting GoPay server..."
	@go run ./cmd/main.go

run-docker: ## Run with Docker Compose
	@echo "🐳 Starting with Docker Compose..."
	@docker-compose up -d

# Development commands
deps: ## Download dependencies
	@echo "📦 Downloading dependencies..."
	@go mod download
	@go mod tidy

format: ## Format code
	@echo "🎨 Formatting code..."
	@go fmt ./...

lint: ## Run linter
	@echo "🔍 Running linter..."
	@golangci-lint run

clean: ## Clean build artifacts
	@echo "🧹 Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@docker-compose down

# Setup commands
setup: deps ## Setup development environment
	@echo "⚙️  Setting up development environment..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "✅ Development environment ready!"

setup-env: ## Create example environment file
	@echo "📝 Creating .env file from example..."
	@cp .env.example .env
	@echo "✅ Please edit .env file with your configuration"

# Database/Migration commands (for future use)
migrate-up: ## Run database migrations up
	@echo "⬆️  Running migrations up..."
	# Add migration command when database is implemented

migrate-down: ## Run database migrations down
	@echo "⬇️  Running migrations down..."
	# Add migration command when database is implemented

# CI/CD helpers
ci-test: ## Run tests in CI environment
	@echo "🤖 Running CI tests..."
	@go test -race -coverprofile=coverage.out -covermode=atomic ./...

ci-build: ## Build for CI environment
	@echo "🤖 Building for CI..."
	@CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/gopay ./cmd/main.go

# Release commands
release: clean build ## Create release build
	@echo "🎉 Creating release..."
	@mkdir -p release/
	@cp bin/gopay release/
	@cp README.md release/
	@cp LICENSE release/
	@tar -czf release/gopay-$(shell date +%Y%m%d).tar.gz -C release/ .
	@echo "✅ Release created in release/ directory"

# Example/Demo commands
demo: ## Run demo with example data
	@echo "🎭 Running demo..."
	@go run ./examples/iyzico_example.go

# Security commands
security-scan: ## Run security scan
	@echo "🔒 Running security scan..."
	@gosec ./...

# Help for integration tests
integration-help: ## Show integration test setup help
	@echo "🧪 Integration Test Setup:"
	@echo ""
	@echo "1. Get İyzico sandbox credentials:"
	@echo "   - Sign up at https://sandbox-merchant.iyzipay.com/"
	@echo "   - Get your API key and secret key"
	@echo ""
	@echo "2. Set environment variables:"
	@echo "   export IYZICO_TEST_ENABLED=true"
	@echo "   export IYZICO_TEST_API_KEY=your_sandbox_api_key"
	@echo "   export IYZICO_TEST_SECRET_KEY=your_sandbox_secret_key"
	@echo ""
	@echo "3. Run integration tests:"
	@echo "   make test-integration"
	@echo "   # or"
	@echo "   make test-iyzico-integration"
	@echo ""
	@echo "Test cards available:"
	@echo "   Success: 5528790000000008"
	@echo "   Insufficient funds: 5528790000000016"
	@echo "   Invalid card: 5528790000000032"

# Quick development workflow
dev: format lint test ## Run development workflow (format, lint, test)
	@echo "✅ Development workflow completed!"

# OpenSearch related commands
opensearch-start: ## Start OpenSearch with Docker
	@echo "Starting OpenSearch..."
	docker-compose up -d opensearch
	@echo "✅ OpenSearch started at http://localhost:9200"

opensearch-stop: ## Stop OpenSearch
	@echo "Stopping OpenSearch..."
	docker-compose stop opensearch
	@echo "✅ OpenSearch stopped"

opensearch-status: ## Check OpenSearch status
	@echo "Checking OpenSearch status..."
	@curl -s http://localhost:9200/_cluster/health | jq . || echo "❌ OpenSearch not responding"

logs-query: ## Query recent payment logs (requires provider parameter)
	@if [ -z "$(PROVIDER)" ]; then \
		echo "Usage: make logs-query PROVIDER=iyzico"; \
		echo "Available providers: iyzico, ozanpay"; \
		exit 1; \
	fi
	@echo "Querying logs for provider: $(PROVIDER)"
	@curl -s "http://localhost:9200/gopay-$(PROVIDER)-logs/_search?size=10&sort=timestamp:desc" | jq .

# Docker commands
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t gopay:latest .
	@echo "✅ Docker image built: gopay:latest"

docker-run: ## Run with Docker Compose
	@echo "Starting services with Docker Compose..."
	docker-compose up -d
	@echo "✅ Services started"

docker-stop: ## Stop Docker Compose services
	@echo "Stopping Docker Compose services..."
	docker-compose down
	@echo "✅ Services stopped"

docker-logs: ## Show Docker logs
	docker-compose logs -f

# CI/CD helpers
ci-test: ## Run tests in CI environment
	@echo "Running CI tests..."
	go test -race -coverprofile=coverage.out -covermode=atomic ./...

ci-build: ## Build for CI/CD
	@echo "Building for CI/CD..."
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-s -w" -o bin/gopay ./cmd/main.go

# Integration test setup
integration-help: ## Show integration test setup instructions
	@echo ""
	@echo "📋 Integration Test Setup"
	@echo "========================="
	@echo ""
	@echo "To run integration tests, you need to set up provider credentials:"
	@echo ""
	@echo "1. İyzico Integration Tests:"
	@echo "   export IYZICO_API_KEY=sandbox-your-api-key"
	@echo "   export IYZICO_SECRET_KEY=sandbox-your-secret-key"
	@echo "   export IYZICO_ENVIRONMENT=sandbox"
	@echo ""
	@echo "2. Enable test mode:"
	@echo "   export IYZICO_TEST_ENABLED=true"
	@echo ""
	@echo "3. Run tests:"
	@echo "   make test-integration"
	@echo "   # or specific provider:"
	@echo "   make test-iyzico"
	@echo ""
	@echo "⚠️  Note: These tests use real API endpoints (sandbox) and may incur charges"
	@echo "   or count against your API rate limits."
	@echo ""

help: ## Display available commands
	@echo ""
	@echo "🚀 GoPay Development Commands"
	@echo "============================="
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "\033[36m%-20s\033[0m %s\n", "Command", "Description"} /^[a-zA-Z_-]+:.*?##/ { printf "\033[36m%-20s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@echo ""
	@echo "Examples:"
	@echo "  make dev                    # Run complete development workflow"
	@echo "  make test-integration       # Run integration tests"
	@echo "  make opensearch-start       # Start OpenSearch for logging"
	@echo "  make logs-query PROVIDER=iyzico  # Query logs"
	@echo "" 