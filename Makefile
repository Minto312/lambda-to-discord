# Lambda to Discord Webhook Makefile

# Variables
BINARY_NAME := bootstrap
FUNCTION_ZIP := function.zip
BUILD_TAGS := lambda
GOOS := linux
GOARCH := amd64

# Default target
.PHONY: all
all: build

# Build the Lambda function binary
.PHONY: build
build:
	@echo "Building Lambda function..."
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -tags $(BUILD_TAGS) -o $(BINARY_NAME)
	@echo "Build completed: $(BINARY_NAME)"

# Create deployment package
.PHONY: package
package: build
	@echo "Creating deployment package..."
	zip $(FUNCTION_ZIP) $(BINARY_NAME)
	@echo "Package created: $(FUNCTION_ZIP)"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test ./...

# Run tests with verbose output
.PHONY: test-verbose
test-verbose:
	@echo "Running tests with verbose output..."
	go test -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -cover ./...

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY_NAME) $(FUNCTION_ZIP)
	@echo "Clean completed"

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run linter (if golangci-lint is installed)
.PHONY: lint
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Run vet
.PHONY: vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Check code quality (fmt + vet + lint)
.PHONY: check
check: fmt vet lint
	@echo "Code quality check completed"

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Show help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build        - Build the Lambda function binary"
	@echo "  package      - Create deployment package (function.zip)"
	@echo "  test         - Run tests"
	@echo "  test-verbose - Run tests with verbose output"
	@echo "  test-coverage- Run tests with coverage"
	@echo "  clean        - Clean build artifacts"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linter (requires golangci-lint)"
	@echo "  vet          - Run go vet"
	@echo "  check        - Run all code quality checks (fmt + vet + lint)"
	@echo "  deps         - Install dependencies"
	@echo "  help         - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  DISCORD_WEBHOOK_URL - Discord webhook URL (required for testing)"
