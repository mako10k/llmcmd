#!/bin/bash

# Create release archives for llmcmd v3.0.0

set -e

VERSION="3.0.0"
BUILD_DIR="dist"
ARCHIVE_DIR="release"

echo "📦 Creating release archives for v${VERSION}..."

# Clean and create archive directory
rm -rf "${ARCHIVE_DIR}"
mkdir -p "${ARCHIVE_DIR}"

# Copy README and LICENSE for inclusion in archives
cp README.md "${BUILD_DIR}/" 2>/dev/null || echo "README.md not found, skipping"
cp LICENSE "${BUILD_DIR}/" 2>/dev/null || echo "LICENSE not found, skipping"

# Create archives for each platform
create_archive() {
    local binary_path=$1
    local binary_name=$(basename "$binary_path")
    local platform=$(echo "$binary_name" | sed 's/llmcmd-v[0-9.]*-//' | sed 's/\.exe$//')
    
    echo "📦 Creating archive for ${platform}..."
    
    # Use tar.gz for all platforms (more universal)
    cd "${BUILD_DIR}"
    tar -czf "../${ARCHIVE_DIR}/llmcmd-v${VERSION}-${platform}.tar.gz" \
        "$binary_name" README.md 2>/dev/null || \
    tar -czf "../${ARCHIVE_DIR}/llmcmd-v${VERSION}-${platform}.tar.gz" \
        "$binary_name"
    cd ..
    
    echo "✅ Created archive for ${platform}"
}

# Create archives for all binaries
for binary in "${BUILD_DIR}"/llmcmd-v*; do
    if [ -f "$binary" ]; then
        create_archive "$binary"
    fi
done

echo ""
echo "📊 Release Archives Created:"
ls -la "${ARCHIVE_DIR}/"

echo ""
echo "🔧 Archive Summary:"
for archive in "${ARCHIVE_DIR}"/*; do
    if [ -f "$archive" ]; then
        size=$(stat -c%s "$archive" 2>/dev/null || stat -f%z "$archive" 2>/dev/null || echo "unknown")
        echo "  $(basename "$archive"): $(numfmt --to=iec --suffix=B $size 2>/dev/null || echo "${size}B")"
    fi
done

echo ""
echo "🚀 Ready for GitHub release upload!"
echo "📁 Archives in: ${ARCHIVE_DIR}/"
