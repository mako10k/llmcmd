#!/bin/bash

# llmcmd Cross-Platform Build Script
# Builds optimized binaries for multiple platforms

set -e

VERSION="v3.1.1"
BUILD_DIR="release"
LDFLAGS_LLMCMD="-s -w -X main.AppVersion=${VERSION}"
LDFLAGS_LLMSH="-s -w -X github.com/mako10k/llmcmd/internal/llmsh.Version=${VERSION}"

echo "Building llmcmd ${VERSION} for multiple platforms..."

# Clean previous builds
rm -rf ${BUILD_DIR}
mkdir -p ${BUILD_DIR}

# Build function
build_binary() {
    local os=$1
    local arch=$2
    local ext=$3
    local cmd_name=$4
    local ldflags=$5
    
    echo "Building ${cmd_name} for ${os}/${arch}..."
    
    GOOS=${os} GOARCH=${arch} go build \
        -ldflags="${ldflags}" \
        -o "${BUILD_DIR}/${cmd_name}-${VERSION}-${os}-${arch}${ext}" \
        ./cmd/${cmd_name}/
}

# Build llmcmd for multiple platforms
echo "=== Building llmcmd ==="
build_binary "linux" "amd64" "" "llmcmd" "${LDFLAGS_LLMCMD}"
build_binary "linux" "arm64" "" "llmcmd" "${LDFLAGS_LLMCMD}"
build_binary "darwin" "amd64" "" "llmcmd" "${LDFLAGS_LLMCMD}"
build_binary "darwin" "arm64" "" "llmcmd" "${LDFLAGS_LLMCMD}"
build_binary "windows" "amd64" ".exe" "llmcmd" "${LDFLAGS_LLMCMD}"
build_binary "windows" "arm64" ".exe" "llmcmd" "${LDFLAGS_LLMCMD}"

# Build llmsh for multiple platforms
echo "=== Building llmsh ==="
build_binary "linux" "amd64" "" "llmsh" "${LDFLAGS_LLMSH}"
build_binary "linux" "arm64" "" "llmsh" "${LDFLAGS_LLMSH}"
build_binary "darwin" "amd64" "" "llmsh" "${LDFLAGS_LLMSH}"
build_binary "darwin" "arm64" "" "llmsh" "${LDFLAGS_LLMSH}"
build_binary "windows" "amd64" ".exe" "llmsh" "${LDFLAGS_LLMSH}"
build_binary "windows" "arm64" ".exe" "llmsh" "${LDFLAGS_LLMSH}"

echo ""
echo "=== Build Summary ==="
echo "Built binaries in ${BUILD_DIR}:"
ls -lh ${BUILD_DIR}/

echo ""
echo "=== File Sizes ==="
for file in ${BUILD_DIR}/*; do
    echo "$(basename "$file"): $(du -h "$file" | cut -f1)"
done

echo ""
echo "✅ Cross-platform build completed for ${VERSION}"
