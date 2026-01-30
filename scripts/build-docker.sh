#!/bin/bash
# Build Docker images for multiple platforms
# Usage: ./scripts/build-docker.sh [platform]
# Platforms: linux-amd64, linux-arm64, all

set -e

APP_NAME="goimagefinder"
VERSION="${VERSION:-latest}"
REGISTRY="${REGISTRY:-}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed or not in PATH"
    exit 1
fi

# Check if buildx is available for multi-platform builds
BUILDX_AVAILABLE=false
if docker buildx version &> /dev/null; then
    BUILDX_AVAILABLE=true
    log_info "Docker buildx is available for multi-platform builds"
fi

build_linux_amd64() {
    log_info "Building for Linux AMD64..."

    if [ "$BUILDX_AVAILABLE" = true ]; then
        docker buildx build \
            --platform linux/amd64 \
            --tag "${REGISTRY}${APP_NAME}:${VERSION}-linux-amd64" \
            --load \
            -f Dockerfile \
            .
    else
        docker build \
            --tag "${REGISTRY}${APP_NAME}:${VERSION}-linux-amd64" \
            -f Dockerfile \
            .
    fi

    log_info "Linux AMD64 build complete"
}

build_linux_arm64() {
    log_info "Building for Linux ARM64..."

    if [ "$BUILDX_AVAILABLE" = true ]; then
        docker buildx build \
            --platform linux/arm64 \
            --tag "${REGISTRY}${APP_NAME}:${VERSION}-linux-arm64" \
            --load \
            -f Dockerfile \
            .
    else
        log_warn "Docker buildx not available, skipping ARM64 build"
        log_warn "Install docker buildx for cross-platform builds"
        return 1
    fi

    log_info "Linux ARM64 build complete"
}

build_multi_platform() {
    log_info "Building multi-platform image..."

    if [ "$BUILDX_AVAILABLE" = false ]; then
        log_error "Docker buildx is required for multi-platform builds"
        exit 1
    fi

    # Create and use a new builder instance if needed
    if ! docker buildx inspect multiplatform-builder &> /dev/null; then
        log_info "Creating new buildx builder..."
        docker buildx create --name multiplatform-builder --use
    else
        docker buildx use multiplatform-builder
    fi

    docker buildx build \
        --platform linux/amd64,linux/arm64 \
        --tag "${REGISTRY}${APP_NAME}:${VERSION}" \
        --push \
        -f Dockerfile \
        .

    log_info "Multi-platform build complete and pushed to registry"
}

# Main
case "${1:-all}" in
    linux-amd64)
        build_linux_amd64
        ;;
    linux-arm64)
        build_linux_arm64
        ;;
    multi|multiplatform)
        if [ -z "$REGISTRY" ]; then
            log_error "REGISTRY environment variable required for multi-platform push"
            log_info "Example: REGISTRY=docker.io/username/ ./scripts/build-docker.sh multi"
            exit 1
        fi
        build_multi_platform
        ;;
    all)
        build_linux_amd64
        if [ "$BUILDX_AVAILABLE" = true ]; then
            build_linux_arm64
        fi
        ;;
    *)
        echo "Usage: $0 [linux-amd64|linux-arm64|multi|all]"
        echo ""
        echo "Platforms:"
        echo "  linux-amd64  - Build for Linux x86_64"
        echo "  linux-arm64  - Build for Linux ARM64 (requires buildx)"
        echo "  multi        - Build and push multi-arch image (requires REGISTRY env var)"
        echo "  all          - Build all available platforms locally"
        echo ""
        echo "Environment variables:"
        echo "  VERSION  - Image version tag (default: latest)"
        echo "  REGISTRY - Docker registry prefix (e.g., docker.io/username/)"
        exit 1
        ;;
esac

log_info "Done!"
