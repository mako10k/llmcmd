# llmcmd Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build parameters
BINARY_NAME=llmcmd
BINARY_PATH=./cmd/llmcmd
BUILD_DIR=build
DIST_DIR=dist

# Version info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0-dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')

# Build flags
LDFLAGS=-ldflags "-X 'main.AppVersion=$(VERSION)' -X 'main.BuildCommit=$(COMMIT)' -X 'main.BuildTime=$(BUILD_TIME)' -w -s"

# Platform targets
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build clean test install uninstall dist release help

all: build

## Build commands
build: ## Build the binary for current platform
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(BINARY_PATH)

build-debug: ## Build with debug info
	@echo "Building debug version..."
	$(GOBUILD) -o $(BINARY_NAME) $(BINARY_PATH)

clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)

## Test commands
test: ## Run tests
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

## Installation commands
install: build ## Install system-wide (requires sudo)
	@echo "Installing $(BINARY_NAME) system-wide..."
	sudo ./$(BINARY_NAME) --install

uninstall: ## Uninstall system-wide (requires sudo)
	@echo "Uninstalling $(BINARY_NAME)..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)

## Distribution commands
dist: ## Build for all platforms
	@echo "Building for all platforms..."
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		OS=$$(echo $$platform | cut -d'/' -f1); \
		ARCH=$$(echo $$platform | cut -d'/' -f2); \
		OUTPUT=$(DIST_DIR)/$(BINARY_NAME)-$$OS-$$ARCH; \
		if [ "$$OS" = "windows" ]; then OUTPUT=$$OUTPUT.exe; fi; \
		echo "Building $$platform -> $$OUTPUT"; \
		GOOS=$$OS GOARCH=$$ARCH $(GOBUILD) $(LDFLAGS) -o $$OUTPUT $(BINARY_PATH); \
	done

release: dist ## Create release with checksums
	@echo "Creating release $(VERSION)..."
	@cd $(DIST_DIR) && sha256sum * > checksums.txt
	@echo "Release files created in $(DIST_DIR)/"
	@ls -la $(DIST_DIR)/

## Development commands
dev-setup: ## Setup development environment
	$(GOMOD) download
	$(GOGET) -u golang.org/x/tools/cmd/goimports

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@goimports -w .

lint: ## Run linter
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## Usage examples
examples: ## Show usage examples
	@echo "Usage examples:"
	@echo ""
	@echo "Basic usage:"
	@echo "  ./$(BINARY_NAME) 'count lines in this file' < input.txt"
	@echo "  echo 'data' | ./$(BINARY_NAME) 'process this'"
	@echo ""
	@echo "File processing:"
	@echo "  ./$(BINARY_NAME) -i data.csv 'extract names column'"
	@echo "  ./$(BINARY_NAME) -i logs.txt -o summary.txt 'summarize errors'"
	@echo ""
	@echo "Environment variables:"
	@echo "  export OPENAI_API_KEY=your_key"
	@echo "  export LLMCMD_MODEL=gpt-4o-mini"
	@echo "  ./$(BINARY_NAME) 'your task'"

## Help
help: ## Show this help
	@echo "llmcmd Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Default target
.DEFAULT_GOAL := help
