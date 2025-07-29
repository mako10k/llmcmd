#!/bin/bash
# llmcmd Installation Script
# Usage: curl -sSL https://raw.githubusercontent.com/mako10k/llmcmd/main/install.sh | bash

set -e

# Configuration
GITHUB_REPO="mako10k/llmcmd"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="$HOME/.config/llmcmd"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Detect OS and architecture
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    case "$arch" in
        x86_64) arch="amd64" ;;
        aarch64) arch="arm64" ;;
        arm64) arch="arm64" ;;
        *) log_error "Unsupported architecture: $arch"; exit 1 ;;
    esac
    
    case "$os" in
        linux) platform="linux-$arch" ;;
        darwin) platform="darwin-$arch" ;;
        *) log_error "Unsupported OS: $os"; exit 1 ;;
    esac
    
    echo "$platform"
}

# Check if running as root for system-wide installation
check_permissions() {
    if [[ "$INSTALL_DIR" == "/usr/local/bin" ]] && [[ $EUID -ne 0 ]]; then
        log_warn "System-wide installation requires sudo privileges"
        log_info "Re-running with sudo..."
        exec sudo bash "$0" "$@"
    fi
}

# Get latest release version
get_latest_version() {
    curl -s "https://api.github.com/repos/$GITHUB_REPO/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"([^"]+)".*/\1/'
}

# Download and install binary
install_binary() {
    local version=$1
    local platform=$2
    local binary_name="llmcmd"
    local download_url="https://github.com/$GITHUB_REPO/releases/download/$version/llmcmd-$platform"
    
    log_info "Downloading llmcmd $version for $platform..."
    
    # Create temporary directory
    local temp_dir=$(mktemp -d)
    trap "rm -rf $temp_dir" EXIT
    
    # Download binary
    if ! curl -sL "$download_url" -o "$temp_dir/$binary_name"; then
        log_error "Failed to download $download_url"
        exit 1
    fi
    
    # Make executable
    chmod +x "$temp_dir/$binary_name"
    
    # Install to system
    log_info "Installing to $INSTALL_DIR/$binary_name..."
    cp "$temp_dir/$binary_name" "$INSTALL_DIR/$binary_name"
    
    log_info "llmcmd installed successfully!"
}

# Create default configuration
create_config() {
    if [[ ! -d "$CONFIG_DIR" ]]; then
        mkdir -p "$CONFIG_DIR"
        log_info "Created config directory: $CONFIG_DIR"
    fi
    
    local config_file="$CONFIG_DIR/config.json"
    if [[ ! -f "$config_file" ]]; then
        cat > "$config_file" << 'EOF'
{
  "model": "gpt-4o-mini",
  "max_tokens": 4096,
  "temperature": 0.1,
  "max_api_calls": 50,
  "timeout_seconds": 300
}
EOF
        log_info "Created default config: $config_file"
    else
        log_info "Config file already exists: $config_file"
    fi
}

# Verify installation
verify_installation() {
    if command -v llmcmd >/dev/null 2>&1; then
        local version=$(llmcmd --version 2>/dev/null || echo "unknown")
        log_info "Installation verified: llmcmd $version"
        log_info ""
        log_info "Usage:"
        log_info "  llmcmd 'your task description'"
        log_info "  echo 'data' | llmcmd 'process this data'"
        log_info "  llmcmd -i input.txt 'analyze this file'"
        log_info ""
        log_info "Environment variables:"
        log_info "  OPENAI_API_KEY=your_api_key"
        log_info "  LLMCMD_MODEL=gpt-4o-mini"
        log_info ""
        log_info "For more help: llmcmd --help"
    else
        log_error "Installation verification failed"
        exit 1
    fi
}

# Main installation flow
main() {
    log_info "Starting llmcmd installation..."
    
    # Check requirements
    if ! command -v curl >/dev/null 2>&1; then
        log_error "curl is required but not installed"
        exit 1
    fi
    
    # Detect platform
    local platform=$(detect_platform)
    log_info "Detected platform: $platform"
    
    # Check permissions
    check_permissions
    
    # Get latest version
    local version=$(get_latest_version)
    if [[ -z "$version" ]]; then
        log_error "Failed to get latest version"
        exit 1
    fi
    log_info "Latest version: $version"
    
    # Install binary
    install_binary "$version" "$platform"
    
    # Create configuration
    create_config
    
    # Verify installation
    verify_installation
    
    log_info "Installation completed successfully!"
}

# Run main function
main "$@"
