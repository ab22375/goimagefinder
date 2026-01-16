# GoImageFinder Dockerfile
# Multi-stage build for smaller final image

# Build stage
FROM golang:1.23-bookworm AS builder

# Install build dependencies for OpenCV and SQLite
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    cmake \
    git \
    pkg-config \
    libopencv-dev \
    libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the webserver
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /app/webserver ./cmd/webserver/

# Runtime stage
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    libopencv-core406 \
    libopencv-imgcodecs406 \
    libopencv-imgproc406 \
    libsqlite3-0 \
    dcraw \
    exiftool \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/webserver /app/webserver

# Create directories for data and config
RUN mkdir -p /data /config /root/.goimagefinder

# Create default config
RUN echo '{"port":8012,"databasePath":"/data/goimagefinder.db","threshold":0.75,"openBrowser":false}' > /root/.goimagefinder/webserver.json

# Expose the default port
EXPOSE 8012

# Volume for persistent data (database) and photos
VOLUME ["/data", "/photos"]

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8012/ || exit 1

# Run the webserver
ENTRYPOINT ["/app/webserver"]
