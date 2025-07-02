#!/bin/bash
set -e

# NAD Controller Installation Script
# Usage: curl -sf https://raw.githubusercontent.com/galamiram/nadctl/main/install.sh | sh

REPO="galamiram/nadctl"
INSTALL_DIR="/usr/local/bin"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
error() {
    echo -e "${RED}Error: $1${NC}" >&2
    exit 1
}

info() {
    echo -e "${GREEN}$1${NC}"
}

warn() {
    echo -e "${YELLOW}$1${NC}"
}

# Detect platform
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    case $os in
        darwin) os="darwin" ;;
        linux) os="linux" ;;
        *) error "Unsupported OS: $os" ;;
    esac
    
    case $arch in
        x86_64) arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *) error "Unsupported architecture: $arch" ;;
    esac
    
    echo "${os}_${arch}"
}

# Get latest release version
get_latest_version() {
    curl -s "https://api.github.com/repos/${REPO}/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"([^"]+)".*/\1/'
}

# Download and install
install_nadctl() {
    local platform=$(detect_platform)
    local version=$(get_latest_version)
    
    if [ -z "$version" ]; then
        error "Could not determine latest version"
    fi
    
    info "Installing nadctl $version for $platform..."
    
    local download_url="https://github.com/${REPO}/releases/download/${version}/nadctl_${version}_${platform}.tar.gz"
    local tmp_dir=$(mktemp -d)
    local archive="$tmp_dir/nadctl.tar.gz"
    
    # Download
    info "Downloading from $download_url"
    if ! curl -L -o "$archive" "$download_url"; then
        error "Failed to download nadctl"
    fi
    
    # Extract
    info "Extracting..."
    if ! tar -xzf "$archive" -C "$tmp_dir"; then
        error "Failed to extract archive"
    fi
    
    # Install
    if [ -w "$INSTALL_DIR" ]; then
        mv "$tmp_dir/nadctl" "$INSTALL_DIR/nadctl"
    else
        info "Installing to $INSTALL_DIR (requires sudo)..."
        sudo mv "$tmp_dir/nadctl" "$INSTALL_DIR/nadctl"
        sudo chmod +x "$INSTALL_DIR/nadctl"
    fi
    
    # Cleanup
    rm -rf "$tmp_dir"
    
    # Verify installation
    if command -v nadctl >/dev/null 2>&1; then
        info "✅ nadctl installed successfully!"
        info "Run 'nadctl --help' to get started"
        info "Try 'nadctl tui' for the interactive interface"
    else
        warn "Installation completed, but nadctl not found in PATH"
        warn "You may need to add $INSTALL_DIR to your PATH"
    fi
}

# Main
main() {
    info "NAD Controller Installation Script"
    info "=================================="
    
    # Check dependencies
    if ! command -v curl >/dev/null 2>&1; then
        error "curl is required but not installed"
    fi
    
    if ! command -v tar >/dev/null 2>&1; then
        error "tar is required but not installed"
    fi
    
    install_nadctl
}

main "$@"