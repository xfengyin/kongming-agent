.PHONY: build test lint fmt clean run run-example docker-build docker-push ci help

# Variables
BINARY_NAME=kongming
BUILD_DIR=./bin
GO=go
GOFLAGS=-ldflags="-s -w"

# Build
build:
	@echo "⚔️  Building..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/kongming
	@echo "✓ Built: $(BUILD_DIR)/$(BINARY_NAME)"

# Test
test:
	@echo "🧪 Running tests..."
	$(GO) test -v -race -cover ./...

# Coverage
cover:
	@echo "📊 Generating coverage report..."
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

# Lint
lint:
	@echo "🔍 Running linters..."
	golangci-lint run ./...

# Format
fmt:
	@echo "✨ Formatting code..."
	$(GO) fmt ./...
	gofumpt -w .

# Clean
clean:
	@echo "🧹 Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "✓ Cleaned"

# Run
run: build
	@echo "🚀 Starting Kongming..."
	$(BUILD_DIR)/$(BINARY_NAME)

# Run Example
run-example:
	@echo "📖 Running quickstart example..."
	$(GO) run ./examples/quickstart/main.go

# Docker Build
docker-build:
	@echo "🐳 Building Docker image..."
	docker build -t zhuge/kongming:latest .

# Docker Run
docker-run:
	@echo "🐳 Running Docker container..."
	docker run -p 8080:8080 -p 9090:9090 zhuge/kongming:latest

# Docker Push
docker-push:
	@echo "📤 Pushing Docker image..."
	docker push zhuge/kongming:latest

# CI (full pipeline)
ci: fmt lint test build
	@echo "✅ All checks passed!"

# Help
help:
	@echo "Kongming Makefile Commands"
	@echo "========================="
	@echo "make build        - Build the binary"
	@echo "make test         - Run tests"
	@echo "make cover        - Generate coverage report"
	@echo "make lint         - Run linters"
	@echo "make fmt          - Format code"
	@echo "make clean        - Clean build artifacts"
	@echo "make run          - Build and run"
	@echo "make run-example  - Run quickstart example"
	@echo "make docker-build - Build Docker image"
	@echo "make docker-run   - Run Docker container"
	@echo "make ci           - Run full CI pipeline"
