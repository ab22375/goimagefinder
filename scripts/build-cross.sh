#!/bin/bash
# Cross-compile Go binaries for multiple platforms
# Usage: ./scripts/build-cross.sh [platform]
# Platforms: linux, darwin, windows, all

set -e

APP_NAME="goimagefinder"
VERSION="${VERSION:-1.0.0}"
BUILD_DIR="./dist"
LDFLAGS="-s -w -X main.Version=${VERSION}"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Clean and create build directory
prepare_build() {
    log_info "Preparing build directory..."
    mkdir -p "${BUILD_DIR}"
}

# Build for a specific platform
build_platform() {
    local os=$1
    local arch=$2
    local ext=$3

    local output="${BUILD_DIR}/${APP_NAME}-${os}-${arch}${ext}"

    log_info "Building for ${os}/${arch}..."

    GOOS=${os} GOARCH=${arch} CGO_ENABLED=0 go build \
        -ldflags="${LDFLAGS}" \
        -o "${output}" \
        ./main.go

    log_info "Built: ${output}"
}

# Build for Linux
build_linux() {
    build_platform "linux" "amd64" ""
    build_platform "linux" "arm64" ""
    build_platform "linux" "arm" ""  # 32-bit ARM for Raspberry Pi
}

# Build for macOS
build_darwin() {
    build_platform "darwin" "amd64" ""
    build_platform "darwin" "arm64" ""  # Apple Silicon
}

# Build for Windows
build_windows() {
    build_platform "windows" "amd64" ".exe"
    build_platform "windows" "arm64" ".exe"
}

# Create checksums
create_checksums() {
    log_info "Creating checksums..."
    cd "${BUILD_DIR}"

    if command -v sha256sum &> /dev/null; then
        sha256sum ${APP_NAME}-* > checksums.txt
    elif command -v shasum &> /dev/null; then
        shasum -a 256 ${APP_NAME}-* > checksums.txt
    else
        log_warn "No sha256sum or shasum found, skipping checksums"
    fi

    cd ..
    log_info "Checksums created: ${BUILD_DIR}/checksums.txt"
}

# Create archives
create_archives() {
    log_info "Creating release archives..."
    cd "${BUILD_DIR}"

    for binary in ${APP_NAME}-*; do
        # Skip checksums file and existing archives
        [[ "$binary" == *.txt ]] && continue
        [[ "$binary" == *.tar.gz ]] && continue
        [[ "$binary" == *.zip ]] && continue

        if [[ "$binary" == *windows* ]]; then
            # Create zip for Windows
            zip "${binary%.exe}.zip" "$binary"
            log_info "Created: ${binary%.exe}.zip"
        else
            # Create tar.gz for Unix
            tar -czvf "${binary}.tar.gz" "$binary"
            log_info "Created: ${binary}.tar.gz"
        fi
    done

    cd ..
}

# Main
prepare_build

case "${1:-all}" in
    linux)
        build_linux
        ;;
    darwin|macos)
        build_darwin
        ;;
    windows)
        build_windows
        ;;
    all)
        build_linux
        build_darwin
        build_windows
        create_checksums
        create_archives
        ;;
    *)
        echo "Usage: $0 [linux|darwin|windows|all]"
        echo ""
        echo "Platforms:"
        echo "  linux   - Build for Linux (amd64, arm64, arm)"
        echo "  darwin  - Build for macOS (amd64, arm64)"
        echo "  windows - Build for Windows (amd64, arm64)"
        echo "  all     - Build for all platforms and create archives"
        echo ""
        echo "Environment variables:"
        echo "  VERSION - Binary version (default: 1.0.0)"
        exit 1
        ;;
esac

log_info "Build complete! Binaries are in ${BUILD_DIR}/"
ls -la "${BUILD_DIR}/"
