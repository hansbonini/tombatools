# TombaTools Makefile
# Use with: make <target>

.PHONY: help build test lint clean release dev install deps security

# Default target
help:
	@echo "TombaTools Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  help      - Show this help message"
	@echo "  build     - Build the binary for current platform"
	@echo "  test      - Run all tests"
	@echo "  lint      - Run linters"
	@echo "  clean     - Clean build artifacts"
	@echo "  release   - Build for all platforms"
	@echo "  dev       - Build and run in development mode"
	@echo "  install   - Install dependencies"
	@echo "  deps      - Update dependencies"
	@echo "  security  - Run security scans"

# Variables
BINARY_NAME=tombatools
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT=$(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags="-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Build for current platform
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) .

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linters
lint:
	@echo "Running linters..."
	golangci-lint run ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME).exe
	rm -f coverage.out coverage.html
	rm -rf dist/
	rm -rf build/

# Build for all platforms
release: clean
	@echo "Building release binaries..."
	mkdir -p dist/
	
	# Windows
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)_windows_amd64.exe .
	
	# Linux
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)_linux_amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)_linux_arm64 .
	
	# macOS
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)_darwin_amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)_darwin_arm64 .
	
	@echo "Release binaries built in dist/"
	@ls -la dist/

# Development build and run
dev: build
	@echo "Running in development mode..."
	./$(BINARY_NAME) --help

# Install dependencies
install:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Update dependencies
deps:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

# Run security scans
security:
	@echo "Running security scans..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not installed. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi
	
	@if command -v nancy >/dev/null 2>&1; then \
		nancy sleuth; \
	else \
		echo "nancy not installed. Install with: go install github.com/sonatypecommunity/nancy@latest"; \
	fi

# Install development tools
tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	go install github.com/sonatypecommunity/nancy@latest
