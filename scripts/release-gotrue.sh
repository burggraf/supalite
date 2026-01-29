#!/bin/bash
set -e

# Script to build and upload GoTrue binaries to GitHub releases
# Usage: ./scripts/release-gotrue.sh [version]
# If no version is provided, it will use GOTRUE_VERSION from Makefile

VERSION=${1:-""}
if [ -z "$VERSION" ]; then
    # Extract version from Makefile
    VERSION=$(grep "GOTRUE_VERSION" Makefile | grep "?" | cut -d' ' -f3)
fi

echo "=========================================="
echo "GoTrue Release Script"
echo "=========================================="
echo "Version: $VERSION"
echo ""

# Check if gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo "Error: GitHub CLI (gh) is not installed"
    echo "Install it from: https://cli.github.com/"
    exit 1
fi

# Check if user is authenticated
if ! gh auth status &> /dev/null; then
    echo "Error: Not authenticated with GitHub CLI"
    echo "Run: gh auth login"
    exit 1
fi

# Get the repository
REPO=$(git remote get-url origin | sed 's/git@github.com:/\//' | sed 's/.git$//')
if [ -z "$REPO" ]; then
    REPO="burggraf/supalite"
fi

echo "Repository: $REPO"
echo ""

# Build directory
BUILD_DIR="/tmp/supalite-gotrue-release"
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

# Clone GoTrue source
echo "Cloning supabase/auth repository at tag $VERSION..."
git clone --depth 1 --branch "$VERSION" https://github.com/supabase/auth.git "$BUILD_DIR/source"

# Platforms to build for (can add more in the future)
# Current: macOS ARM64 (Apple Silicon)
# Future: macOS AMD64 (Intel), Linux ARM64, Linux AMD64, Windows ARM64, Windows AMD64
declare -A PLATFORMS
PLATFORMS["darwin-arm64"]="gotrue-darwin-arm64"
# PLATFORMS["darwin-amd64"]="gotrue-darwin-amd64"
# PLATFORMS["linux-arm64"]="gotrue-linux-arm64"
# PLATFORMS["linux-amd64"]="gotrue-linux-amd64"
# PLATFORMS["windows-arm64"]="gotrue-windows-arm64.exe"
# PLATFORMS["windows-amd64"]="gotrue-windows-amd64.exe"

echo ""
echo "Building GoTrue binaries..."
echo ""

# Build for each platform
for PLATFORM_KEY in "${!PLATFORMS[@]}"; do
    BINARY_NAME="${PLATFORMS[$PLATFORM_KEY]}"
    OS="${PLATFORM_KEY%-*}"
    ARCH="${PLATFORM_KEY#*-}"

    echo "Building for $OS-$ARCH ($BINARY_NAME)..."

    cd "$BUILD_DIR/source"
    GOOS=$OS GOARCH=$ARCH make build

    # Move and rename the binary
    mv auth "$BUILD_DIR/$BINARY_NAME"
    chmod +x "$BUILD_DIR/$BINARY_NAME"

    # Show file size
    SIZE=$(ls -lh "$BUILD_DIR/$BINARY_NAME" | awk '{print $5}')
    echo "  Built: $BINARY_NAME ($SIZE)"
done

echo ""
echo "=========================================="
echo "Build Summary"
echo "=========================================="

ls -lh "$BUILD_DIR"/gotrue-*

echo ""
echo "Creating GitHub release..."
echo ""

# Check if release already exists
if gh release view "$VERSION" --repo "$REPO" &> /dev/null; then
    echo "Release $VERSION already exists"
    echo "Updating existing release..."
    gh release delete "$VERSION" --repo "$REPO" --yes
fi

# Create the release
gh release create "$VERSION" \
    --repo "$REPO" \
    --title "GoTrue v$VERSION" \
    --notes "GoTrue auth server binaries for Supalite

**Version:** $VERSION
**Source:** https://github.com/supabase/auth

Supported platforms:
- macOS ARM64 (Apple Silicon)

More platforms coming soon.

**Usage:**
The Supalite app will automatically download the appropriate binary for your platform on first run."

echo ""
echo "Uploading binaries..."

# Upload each binary
for PLATFORM_KEY in "${!PLATFORMS[@]}"; do
    BINARY_NAME="${PLATFORMS[$PLATFORM_KEY]}"
    echo "Uploading $BINARY_NAME..."

    gh release upload "$VERSION" \
        "$BUILD_DIR/$BINARY_NAME" \
        --repo "$REPO" \
        --clobber
done

echo ""
echo "=========================================="
echo "Release Complete!"
echo "=========================================="
echo "Version: $VERSION"
echo "Release URL: https://github.com/$REPO/releases/tag/$VERSION"
echo ""
echo "Binaries uploaded:"
for PLATFORM_KEY in "${!PLATFORMS[@]}"; do
    BINARY_NAME="${PLATFORMS[$PLATFORM_KEY]}"
    SIZE=$(ls -lh "$BUILD_DIR/$BINARY_NAME" | awk '{print $5}')
    echo "  - $BINARY_NAME ($SIZE)"
done
echo ""

# Cleanup
rm -rf "$BUILD_DIR"
