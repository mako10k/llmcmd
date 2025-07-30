#!/bin/bash

# llmcmd Cross-Platform Build Script
# Builds binaries for multiple platforms and creates release archives

set -e

VERSION="3.0.1"
APP_NAME="llmcmd"
BUILD_DIR="release/v${VERSION}"

echo "🚀 Building ${APP_NAME} v${VERSION} for multiple platforms..."

# Create build directory
mkdir -p "${BUILD_DIR}"

# Build targets (OS/ARCH combinations)
declare -a TARGETS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

# Build each target
for target in "${TARGETS[@]}"; do
    OS=$(echo $target | cut -d'/' -f1)
    ARCH=$(echo $target | cut -d'/' -f2)
    
    echo "📦 Building for ${OS}/${ARCH}..."
    
    # Set binary name (add .exe for Windows)
    BINARY_NAME="${APP_NAME}-v${VERSION}-${OS}-${ARCH}"
    if [ "$OS" = "windows" ]; then
        BINARY_NAME="${BINARY_NAME}.exe"
    fi
    
    # Build binary
    GOOS=$OS GOARCH=$ARCH go build -ldflags="-s -w" -o "${BUILD_DIR}/${BINARY_NAME}" ./cmd/llmcmd
    
    # Create tar.gz archive (except for Windows)
    if [ "$OS" != "windows" ]; then
        echo "📁 Creating archive for ${OS}/${ARCH}..."
        tar -czf "${BUILD_DIR}/${APP_NAME}-v${VERSION}-${OS}-${ARCH}.tar.gz" -C "${BUILD_DIR}" "${BINARY_NAME}"
        
        # Calculate SHA256
        cd "${BUILD_DIR}"
        sha256sum "${APP_NAME}-v${VERSION}-${OS}-${ARCH}.tar.gz" >> "checksums.txt"
        cd - > /dev/null
    else
        # Create tar.gz archive for Windows too (zip not available)
        echo "📁 Creating archive for ${OS}/${ARCH}..."
        tar -czf "${BUILD_DIR}/${APP_NAME}-v${VERSION}-${OS}-${ARCH}.tar.gz" -C "${BUILD_DIR}" "${BINARY_NAME}"
        
        # Calculate SHA256
        cd "${BUILD_DIR}"
        sha256sum "${APP_NAME}-v${VERSION}-${OS}-${ARCH}.tar.gz" >> "checksums.txt"
        cd - > /dev/null
    fi
    
    echo "✅ Completed ${OS}/${ARCH}"
done

# Generate release information
cat > "${BUILD_DIR}/README.md" << EOF
# llmcmd v${VERSION} Release Binaries

## Download Instructions

### Linux AMD64
\`\`\`bash
wget https://github.com/mako10k/llmcmd/releases/download/v${VERSION}/llmcmd-v${VERSION}-linux-amd64.tar.gz
tar -xzf llmcmd-v${VERSION}-linux-amd64.tar.gz
chmod +x llmcmd-v${VERSION}-linux-amd64
sudo mv llmcmd-v${VERSION}-linux-amd64 /usr/local/bin/llmcmd
\`\`\`

### Linux ARM64
\`\`\`bash
wget https://github.com/mako10k/llmcmd/releases/download/v${VERSION}/llmcmd-v${VERSION}-linux-arm64.tar.gz
tar -xzf llmcmd-v${VERSION}-linux-arm64.tar.gz
chmod +x llmcmd-v${VERSION}-linux-arm64
sudo mv llmcmd-v${VERSION}-linux-arm64 /usr/local/bin/llmcmd
\`\`\`

### macOS AMD64 (Intel)
\`\`\`bash
wget https://github.com/mako10k/llmcmd/releases/download/v${VERSION}/llmcmd-v${VERSION}-darwin-amd64.tar.gz
tar -xzf llmcmd-v${VERSION}-darwin-amd64.tar.gz
chmod +x llmcmd-v${VERSION}-darwin-amd64
sudo mv llmcmd-v${VERSION}-darwin-amd64 /usr/local/bin/llmcmd
\`\`\`

### macOS ARM64 (Apple Silicon)
\`\`\`bash
wget https://github.com/mako10k/llmcmd/releases/download/v${VERSION}/llmcmd-v${VERSION}-darwin-arm64.tar.gz
tar -xzf llmcmd-v${VERSION}-darwin-arm64.tar.gz
chmod +x llmcmd-v${VERSION}-darwin-arm64
sudo mv llmcmd-v${VERSION}-darwin-arm64 /usr/local/bin/llmcmd
\`\`\`

### Windows AMD64
\`\`\`bash
# Download and extract (using WSL, Git Bash, or similar)
wget https://github.com/mako10k/llmcmd/releases/download/v${VERSION}/llmcmd-v${VERSION}-windows-amd64.tar.gz
tar -xzf llmcmd-v${VERSION}-windows-amd64.tar.gz
# Add the extracted .exe file to your PATH
\`\`\`

## Verification

Verify the download with checksums:
\`\`\`bash
sha256sum -c checksums.txt
\`\`\`

## Installation Test

After installation, verify the version:
\`\`\`bash
llmcmd --version
# Should output: llmcmd version ${VERSION}
\`\`\`
EOF

echo ""
echo "🎉 Build completed successfully!"
echo "📁 Release files created in: ${BUILD_DIR}/"
echo ""
echo "📋 Built artifacts:"
ls -la "${BUILD_DIR}/"
echo ""
echo "🔐 Checksums:"
cat "${BUILD_DIR}/checksums.txt"
