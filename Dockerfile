# GoImageFinder Dockerfile
# Multi-stage build for smaller final image

# Build stage
FROM golang:1.24-bookworm AS builder

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    cmake \
    git \
    pkg-config \
    libsqlite3-dev \
    wget \
    unzip \
    libjpeg-dev \
    libpng-dev \
    libtiff-dev \
    libavcodec-dev \
    libavformat-dev \
    libswscale-dev \
    libv4l-dev \
    libxvidcore-dev \
    libx264-dev \
    libgtk-3-dev \
    libatlas-base-dev \
    gfortran \
    && rm -rf /var/lib/apt/lists/*

# Build OpenCV 4.9.0 from source (required for gocv 0.41.0)
ENV OPENCV_VERSION=4.9.0
WORKDIR /tmp
RUN wget -q -O opencv.zip https://github.com/opencv/opencv/archive/${OPENCV_VERSION}.zip && \
    wget -q -O opencv_contrib.zip https://github.com/opencv/opencv_contrib/archive/${OPENCV_VERSION}.zip && \
    unzip -q opencv.zip && \
    unzip -q opencv_contrib.zip && \
    mkdir -p opencv-${OPENCV_VERSION}/build && \
    cd opencv-${OPENCV_VERSION}/build && \
    cmake -D CMAKE_BUILD_TYPE=RELEASE \
          -D CMAKE_INSTALL_PREFIX=/usr/local \
          -D OPENCV_EXTRA_MODULES_PATH=/tmp/opencv_contrib-${OPENCV_VERSION}/modules \
          -D BUILD_DOCS=OFF \
          -D BUILD_EXAMPLES=OFF \
          -D BUILD_TESTS=OFF \
          -D BUILD_PERF_TESTS=OFF \
          -D BUILD_opencv_java=OFF \
          -D BUILD_opencv_python=OFF \
          -D BUILD_opencv_python2=OFF \
          -D BUILD_opencv_python3=OFF \
          -D WITH_JASPER=OFF \
          -D WITH_TBB=ON \
          -D OPENCV_GENERATE_PKGCONFIG=ON \
          .. && \
    make -j$(nproc) && \
    make install && \
    ldconfig && \
    cd /tmp && \
    rm -rf opencv*

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the webserver
ENV PKG_CONFIG_PATH=/usr/local/lib/pkgconfig
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /app/webserver ./cmd/webserver/

# Runtime stage
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    libjpeg62-turbo \
    libpng16-16 \
    libtiff6 \
    libavcodec59 \
    libavformat59 \
    libswscale6 \
    libgtk-3-0 \
    libsqlite3-0 \
    dcraw \
    exiftool \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Copy OpenCV libraries from builder
COPY --from=builder /usr/local/lib /usr/local/lib
RUN ldconfig

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/webserver /app/webserver

# Create directories for data and config
RUN mkdir -p /data /config /root/.goimagefinder

# Create default config (environment variables override these values)
RUN echo '{"port":8012,"databasePath":"/data/goimagefinder.db","threshold":0.75,"openBrowser":false,"browseRoots":["/photos"]}' > /root/.goimagefinder/webserver.json

# Environment variables for flexible configuration:
#   GOIMAGEFINDER_PORT           - HTTP port (default: 8012)
#   GOIMAGEFINDER_DATABASE_PATH  - Database location (default: /data/goimagefinder.db)
#   GOIMAGEFINDER_BROWSE_ROOTS   - Comma-separated paths (default: /photos)
#   GOIMAGEFINDER_THRESHOLD      - Similarity threshold 0-1 (default: 0.75)
#   GOIMAGEFINDER_OPEN_BROWSER   - Auto-open browser (default: false in Docker)

# Expose the default port
EXPOSE 8012

# Volumes for persistent data and photos
# Mount your photo folders to any path and configure via GOIMAGEFINDER_BROWSE_ROOTS
VOLUME ["/data"]

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8012/ || exit 1

# Run the webserver
ENTRYPOINT ["/app/webserver"]

# =============================================================================
# USAGE EXAMPLES
# =============================================================================
#
# BASIC USAGE (single photo folder):
#   docker run -d -p 8012:8012 \
#     -v /path/to/photos:/photos \
#     -v goimagefinder_data:/data \
#     ab22375/goimagefinder
#
# MULTIPLE PHOTO FOLDERS:
#   docker run -d -p 8012:8012 \
#     -v /Users/me/Photos:/photos \
#     -v /Volumes/External:/external \
#     -v /Volumes/Backup:/backup \
#     -v goimagefinder_data:/data \
#     -e GOIMAGEFINDER_BROWSE_ROOTS="/photos,/external,/backup" \
#     ab22375/goimagefinder
#
# DOCKER COMPOSE (recommended for multiple folders):
#   See docker-compose.yml example below
#
# =============================================================================
