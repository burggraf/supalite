#!/bin/bash
set -e

# Script to build release versions of Supalite for multiple platforms
# Usage: ./scripts/release-supalite.sh [version]
# If no version is provided, it will use git describe

VERSION=${1:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo "=========================================="
echo "Supalite Release Build Script"
echo "=========================================="
echo "Version: $VERSION"
echo "Build Time: $BUILD_TIME"
echo "Git Commit: $GIT_COMMIT"
echo ""

# Build directory
BUILD_DIR="./release"
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# Platforms to build for
declare -a PLATFORMS=(
    "darwin-arm64:supalite-darwin-arm64"
    "darwin-amd64:supalite-darwin-amd64"
    "linux-arm64:supalite-linux-arm64"
    "linux-amd64:supalite-linux-amd64"
    "windows-arm64:supalite-windows-arm64.exe"
    "windows-amd64:supalite-windows-amd64.exe"
)

echo "Building Supalite binaries..."
echo ""

# Build for each platform
for PLATFORM_ENTRY in "${PLATFORMS[@]}"; do
    IFS=':' read -r PLATFORM_PAIR BINARY_NAME <<< "$PLATFORM_ENTRY"
    GOOS="${PLATFORM_PAIR%-*}"
    GOARCH="${PLATFORM_PAIR#*-}"

    echo "Building for $GOOS-$GOARCH ($BINARY_NAME)..."

    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "-X github.com/markb/supalite/cmd.Version=$VERSION -X github.com/markb/supalite/cmd.BuildTime=$BUILD_TIME -X github.com/markb/supalite/cmd.GitCommit=$GIT_COMMIT" \
        -o "$BUILD_DIR/$BINARY_NAME" .

    # Make executable (not for Windows)
    if [[ "$BINARY_NAME" != *.exe ]]; then
        chmod +x "$BUILD_DIR/$BINARY_NAME"
    fi

    # Show file size
    SIZE=$(ls -lh "$BUILD_DIR/$BINARY_NAME" | awk '{print $5}')
    echo "  Built: $BINARY_NAME ($SIZE)"
done

echo ""
echo "=========================================="
echo "Build Summary"
echo "=========================================="

ls -lh "$BUILD_DIR"/supalite-*

echo ""
echo "Total sizes:"
du -sh "$BUILD_DIR"

echo ""
echo "Binaries are ready in: $BUILD_DIR"
echo ""
echo "To test:"
echo "  ./$BUILD_DIR/supalite-darwin-arm64 serve"
echo ""
