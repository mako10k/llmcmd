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
LLMSH_NAME=llmsh
BINARY_PATH=./cmd/llmcmd
BUILD_DIR=build
DIST_DIR=dist

# Rust parameters (optional)
CARGO?=cargo
LLMSH_RS_DIR=./llmsh-rs
VFSD_RS_DIR=./vfsd
LLMSH_RS_BIN=llmsh
VFSD_RS_BIN=vfsd
VFSD_ARTIFACT=vfsd.bin

# Publish/CI toggles (override with `make VAR=...`)
# Default is OPT-OUT: Rust tools are required unless explicitly disabled.
BUILD_RUST ?= 1           # 1 (default): build llmsh/vfsd; 0: skip building Rust helpers
SKIP_POLICY ?= 0          # 1: skip policy checks in `make test`
INSTALL_LLMSH ?= 1        # 1 (default): install llmsh, 0: skip
INSTALL_VFSD ?= 1         # 1 (default): install vfsd, 0: skip

# Version info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0-dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')

# Build flags
LDFLAGS_LLMCMD=-ldflags "-X 'main.AppVersion=$(VERSION)' -X 'main.BuildCommit=$(COMMIT)' -X 'main.BuildTime=$(BUILD_TIME)' -w -s"


# Platform targets
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build build-core build-llmcmd build-llmsh build-vfsd build-debug clean test install install-core uninstall dist release help publish dev-setup fmt lint policy examples

all: build

## Build commands
# Conditionally include Rust helper builds
BUILD_RUST_TARGETS :=
ifeq ($(strip $(BUILD_RUST)),1)
BUILD_RUST_TARGETS += build-llmsh build-vfsd
endif

build: build-llmcmd $(BUILD_RUST_TARGETS) ## Build binaries for current platform (Rust tools ON by default; use BUILD_RUST=0 to opt-out)
	@echo "[build] BUILD_RUST='$(BUILD_RUST)' (trimmed='$(strip $(BUILD_RUST))') -> targets: $(BUILD_RUST_TARGETS)"

.PHONY: build-core
build-core: build-llmcmd ## Build llmcmd only (no Rust tools)

build-llmcmd: ## Build llmcmd binary
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	$(GOBUILD) $(LDFLAGS_LLMCMD) -o $(BINARY_NAME) $(BINARY_PATH)

build-llmsh: ## Build llmsh binary (Rust only; required by default)
	@echo "Building $(LLMSH_NAME) (Rust) $(VERSION)..."
	@if [ -f $(LLMSH_RS_DIR)/Cargo.toml ] && command -v $(CARGO) >/dev/null 2>&1; then \
		echo "Building Rust $(LLMSH_NAME) from $(LLMSH_RS_DIR)..."; \
		(cd $(LLMSH_RS_DIR) && $(CARGO) build --release); \
		if [ -x $(LLMSH_RS_DIR)/target/release/$(LLMSH_RS_BIN) ]; then \
			cp $(LLMSH_RS_DIR)/target/release/$(LLMSH_RS_BIN) ./$(LLMSH_NAME); \
			echo "Built ./$(LLMSH_NAME) from Rust ($(LLMSH_RS_BIN))"; \
		elif [ -x $(LLMSH_RS_DIR)/target/release/llmsh-rs ]; then \
			cp $(LLMSH_RS_DIR)/target/release/llmsh-rs ./$(LLMSH_NAME); \
			echo "Built ./$(LLMSH_NAME) from Rust (llmsh-rs)"; \
		elif [ -x $(LLMSH_RS_DIR)/target/release/llmsh ]; then \
			cp $(LLMSH_RS_DIR)/target/release/llmsh ./$(LLMSH_NAME); \
			echo "Built ./$(LLMSH_NAME) from Rust (llmsh)"; \
		else \
			echo "Error: Rust llmsh artifact not found under $(LLMSH_RS_DIR)/target/release" >&2; exit 1; \
		fi; \
	else \
		echo "Error: Rust project or cargo not available for $(LLMSH_NAME). Rust tools are required by default." >&2; \
		echo "Hint: install Rust/Cargo, or explicitly opt-out via 'make BUILD_RUST=0 ...'" >&2; \
		exit 1; \
	fi

build-vfsd: ## Build vfsd helper (Rust; required by default)
	@if [ -f $(VFSD_RS_DIR)/Cargo.toml ] && command -v $(CARGO) >/dev/null 2>&1; then \
		echo "Building Rust $(VFSD_RS_BIN) from $(VFSD_RS_DIR)..."; \
		(cd $(VFSD_RS_DIR) && $(CARGO) build --release); \
		cp $(VFSD_RS_DIR)/target/release/$(VFSD_RS_BIN) ./$(VFSD_ARTIFACT); \
		echo "Built ./$(VFSD_ARTIFACT)"; \
	else \
		echo "Error: Rust project or cargo not available for $(VFSD_RS_BIN). Rust tools are required by default." >&2; \
		echo "Hint: install Rust/Cargo, or explicitly opt-out via 'make BUILD_RUST=0 ...'" >&2; \
		exit 1; \
	fi

build-debug: ## Build with debug info
	@echo "Building debug version..."
	$(GOBUILD) -o $(BINARY_NAME) $(BINARY_PATH)
	@if [ -f $(LLMSH_RS_DIR)/Cargo.toml ] && command -v $(CARGO) >/dev/null 2>&1; then \
		echo "Building Rust $(LLMSH_NAME) (debug) from $(LLMSH_RS_DIR)..."; \
		(cd $(LLMSH_RS_DIR) && $(CARGO) build); \
		if [ -x $(LLMSH_RS_DIR)/target/debug/$(LLMSH_RS_BIN) ]; then \
			cp $(LLMSH_RS_DIR)/target/debug/$(LLMSH_RS_BIN) ./$(LLMSH_NAME); \
			echo "Built ./$(LLMSH_NAME) (debug) from Rust ($(LLMSH_RS_BIN))"; \
		elif [ -x $(LLMSH_RS_DIR)/target/debug/llmsh-rs ]; then \
			cp $(LLMSH_RS_DIR)/target/debug/llmsh-rs ./$(LLMSH_NAME); \
			echo "Built ./$(LLMSH_NAME) (debug) from Rust (llmsh-rs)"; \
		elif [ -x $(LLMSH_RS_DIR)/target/debug/llmsh ]; then \
			cp $(LLMSH_RS_DIR)/target/debug/llmsh ./$(LLMSH_NAME); \
			echo "Built ./$(LLMSH_NAME) (debug) from Rust (llmsh)"; \
		else \
			echo "Skipping $(LLMSH_NAME) debug build: Rust artifact not found under $(LLMSH_RS_DIR)/target/debug"; \
		fi; \
	else \
		echo "Skipping $(LLMSH_NAME) debug build: Rust project or cargo not available"; \
	fi

clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(LLMSH_NAME)
	rm -f ./$(VFSD_ARTIFACT)
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)

## Test commands
test: ## Run tests (set SKIP_POLICY=1 to skip policy checks)
	@if [ "$(SKIP_POLICY)" != "1" ]; then \
		./scripts/policy_check.sh; \
	else \
		echo "Skipping policy check (SKIP_POLICY=1)"; \
	fi
	$(GOTEST) -v ./...

test-integration: build
	@chmod +x test_integration/virtual_mode_llmsh_test.sh
	@./test_integration/virtual_mode_llmsh_test.sh

.PHONY: test-integration
test-coverage: ## Run tests with coverage
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

## Installation commands
install: build ## Install binaries system-wide (requires sudo)
	@echo "Installing $(BINARY_NAME) system-wide..."
	sudo ./$(BINARY_NAME) --install
	@val_llmsh='$(INSTALL_LLMSH)'; val_llmsh_trim=$$(printf '%s' "$$val_llmsh" | tr -d '[:space:]'); \
	if [ "$$val_llmsh_trim" = "1" ]; then \
		if [ -f ./$(LLMSH_NAME) ]; then \
			echo "Installing $(LLMSH_NAME) to /usr/local/bin..."; \
			sudo cp ./$(LLMSH_NAME) /usr/local/bin/$(LLMSH_NAME); \
			sudo chmod +x /usr/local/bin/$(LLMSH_NAME); \
		else \
			echo "Error: './$(LLMSH_NAME)' not found. Build it (make build-llmsh) or opt-out: INSTALL_LLMSH=0" >&2; \
			exit 1; \
		fi; \
	else \
		echo "Skipping $(LLMSH_NAME) install: INSTALL_LLMSH='$$val_llmsh' (trimmed='$$val_llmsh_trim') (explicit opt-out)"; \
	fi
	@val_vfsd='$(INSTALL_VFSD)'; val_vfsd_trim=$$(printf '%s' "$$val_vfsd" | tr -d '[:space:]'); \
	if [ "$$val_vfsd_trim" = "1" ]; then \
		if [ -f ./$(VFSD_ARTIFACT) ]; then \
			echo "Installing $(VFSD_RS_BIN) to /usr/local/bin..."; \
			sudo cp ./$(VFSD_ARTIFACT) /usr/local/bin/$(VFSD_RS_BIN); \
			sudo chmod +x /usr/local/bin/$(VFSD_RS_BIN); \
		else \
			echo "Error: './$(VFSD_ARTIFACT)' not found. Build it (make build-vfsd) or opt-out: INSTALL_VFSD=0" >&2; \
			exit 1; \
		fi; \
	else \
		echo "Skipping $(VFSD_RS_BIN) install: INSTALL_VFSD='$$val_vfsd' (trimmed='$$val_vfsd_trim') (explicit opt-out)"; \
	fi

.PHONY: install-core
install-core: build-llmcmd ## Install llmcmd only (no llmsh/vfsd)
	@echo "Installing $(BINARY_NAME) system-wide..."
	sudo ./$(BINARY_NAME) --install

uninstall: ## Uninstall both binaries system-wide (requires sudo)
	@echo "Uninstalling $(BINARY_NAME)..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "Uninstalling $(LLMSH_NAME)..."
	sudo rm -f /usr/local/bin/$(LLMSH_NAME)
	@echo "Uninstalling $(VFSD_RS_BIN) (if present)..."
	sudo rm -f /usr/local/bin/$(VFSD_RS_BIN)

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
		GOOS=$$OS GOARCH=$$ARCH $(GOBUILD) $(LDFLAGS_LLMCMD) -o $$OUTPUT $(BINARY_PATH); \
	done

release: dist ## Create release with checksums
	@echo "Creating release $(VERSION)..."
	@cd $(DIST_DIR) && sha256sum * > checksums.txt
	@echo "Release files created in $(DIST_DIR)/"
	@ls -la $(DIST_DIR)/

.PHONY: publish
publish: ## Build release artifacts for llmcmd only (no policy, no Rust tools)
	@echo "Publishing core llmcmd (BUILD_RUST=0, skipping tests/policy)..."
	$(MAKE) BUILD_RUST=0 clean release

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

.PHONY: policy
policy: ## Run security/architecture policy checks (no external exec, no direct FS)
	@./scripts/policy_check.sh

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
	@echo ""
	@echo "Toggles (override like VAR=0):"
	@echo "  BUILD_RUST=1     Build Rust tools (llmsh/vfsd). Set BUILD_RUST=0 to opt-out."
	@echo "  SKIP_POLICY=0    Skip policy check in 'make test' when set to 1"
	@echo "  INSTALL_LLMSH=1  Install llmsh by default; set INSTALL_LLMSH=0 to skip"
	@echo "  INSTALL_VFSD=1   Install vfsd by default; set INSTALL_VFSD=0 to skip"

# Default target
.DEFAULT_GOAL := help
