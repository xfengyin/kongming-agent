.PHONY: build test cover coverage-gate lint fmt clean ci proto help

BINARY_NAME=kongming
SERVER_NAME=kongming-server
BUILD_DIR=./bin
GO=go
GOFLAGS=-ldflags="-s -w"

build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(SERVER_NAME) ./cmd/kongming-server
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/kongming
	@echo "Built: $(BUILD_DIR)/$(SERVER_NAME), $(BUILD_DIR)/$(BINARY_NAME)"

test:
	@echo "Running tests..."
	$(GO) test -v -race -cover ./...

cover:
	@echo "Coverage report..."
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

coverage-gate:
	@echo "Coverage gate..."
	@$(GO) test -coverprofile=coverage.out ./... > /dev/null 2>&1
	@COV=$$($(GO) tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | tr -d '%'); \
	echo "Coverage: $$COV%"; \
	if [ $${COV%.*} -lt 80 ]; then echo "FAIL: < 80%"; exit 1; fi

lint:
	@echo "Running linters..."
	golangci-lint run --timeout=5m ./...

fmt:
	@echo "Formatting..."
	$(GO) fmt ./...
	gofumpt -w .

clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR) coverage.out coverage.html

proto:
	@echo "Generating proto..."
	cd api/proto && buf generate

ci: fmt lint coverage-gate build

help:
	@echo "make build         - Build all binaries"
	@echo "make test          - Run all tests with race"
	@echo "make cover         - HTML coverage report"
	@echo "make coverage-gate - Enforce 80% coverage"
	@echo "make lint          - Run golangci-lint"
	@echo "make fmt           - Format code"
	@echo "make proto         - Generate gRPC code"
	@echo "make ci            - Full CI pipeline"
