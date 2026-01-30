# Multi-stage build for minimal final image
# Stage 1: Build the Go binary
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -extldflags '-static'" \
    -o /goimagefinder ./main.go

# Stage 2: Minimal runtime image
FROM alpine:3.21

# Install runtime dependencies for RAW image processing
# Note: dcraw is deprecated, using libraw (dcraw_emu) instead
RUN apk add --no-cache \
    perl-image-exiftool \
    libraw-tools \
    ca-certificates \
    tzdata

# Create non-root user for security
RUN adduser -D -u 1000 appuser

# Copy binary from builder
COPY --from=builder /goimagefinder /usr/local/bin/goimagefinder

# Create data directories
RUN mkdir -p /data/images /data/db && \
    chown -R appuser:appuser /data

# Switch to non-root user
USER appuser

# Set working directory
WORKDIR /data

# Default environment variables
ENV GOIMAGEFINDER_DB=/data/db/images.db

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD goimagefinder info --database=${GOIMAGEFINDER_DB} || exit 1

# Default command shows help
ENTRYPOINT ["goimagefinder"]
CMD ["--help"]
