#!/bin/bash

# llmcmd v3.0.0 Cross-platform Build Script
# Builds binaries for multiple platforms

set -e

VERSION="3.0.0"
APP_NAME="llmcmd"
BUILD_DIR="dist"
MAIN_FILE="cmd/llmcmd/main.go"

echo "🚀 Building ${APP_NAME} v${VERSION} for multiple platforms..."

# Clean previous builds
rm -rf "${BUILD_DIR}"
mkdir -p "${BUILD_DIR}"

# Platform targets
declare -a PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
    "windows/arm64"
)

# Build function
build_platform() {
    local platform=$1
    local GOOS=$(echo $platform | cut -d'/' -f1)
    local GOARCH=$(echo $platform | cut -d'/' -f2)
    
    local output_name="${APP_NAME}"
    if [ "$GOOS" = "windows" ]; then
        output_name="${APP_NAME}.exe"
    fi
    
    local output_path="${BUILD_DIR}/${APP_NAME}-v${VERSION}-${GOOS}-${GOARCH}"
    if [ "$GOOS" = "windows" ]; then
        output_path="${output_path}.exe"
    fi
    
    echo "📦 Building ${GOOS}/${GOARCH}..."
    
    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags="-s -w -X main.AppVersion=${VERSION}" \
        -o "${output_path}" \
        "${MAIN_FILE}"
    
    echo "✅ Built: ${output_path}"
}

# Build for all platforms
for platform in "${PLATFORMS[@]}"; do
    build_platform "$platform"
done

echo ""
echo "🎉 Cross-platform build completed!"
echo "📁 Build artifacts in: ${BUILD_DIR}/"

# Show build results
ls -la "${BUILD_DIR}/"

echo ""
echo "📊 Build Summary:"
for file in "${BUILD_DIR}"/*; do
    if [ -f "$file" ]; then
        size=$(stat -c%s "$file" 2>/dev/null || stat -f%z "$file" 2>/dev/null || echo "unknown")
        echo "  $(basename "$file"): $(numfmt --to=iec --suffix=B $size 2>/dev/null || echo "${size}B")"
    fi
done

echo ""
echo "🔧 Next steps:"
echo "1. Test binaries on target platforms"
echo "2. Create GitHub release for v${VERSION}"
echo "3. Upload artifacts to release"
echo "4. Update documentation"
