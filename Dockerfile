# Nereus SNMP Trap Alerting System - Multi-stage Docker Build
# This Dockerfile creates an optimized production image for Nereus

# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    make

# Set working directory
WORKDIR /app

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build arguments for version information
ARG VERSION=dev
ARG BUILD_TIME
ARG COMMIT_HASH

# Set build time if not provided
RUN if [ -z "$BUILD_TIME" ]; then \
        BUILD_TIME=$(date +%Y%m%d%H%M%S); \
    fi

# Set commit hash if not provided
RUN if [ -z "$COMMIT_HASH" ]; then \
        COMMIT_HASH=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown"); \
    fi

# Build the binary with version information as specified in requirements
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X main.version=${VERSION} -X main.buildTime=${BUILD_TIME} -X main.commitHash=${COMMIT_HASH} -w -s" \
    -a -installsuffix cgo \
    -o nereus ./main.go

# Verify the binary
RUN ./nereus --version

# Runtime stage
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    sqlite \
    && rm -rf /var/cache/apk/*

# Create non-root user for security
RUN addgroup -g 1000 nereus && \
    adduser -D -s /bin/sh -u 1000 -G nereus nereus

# Create necessary directories
RUN mkdir -p /etc/nereus /var/lib/nereus /var/log/nereus && \
    chown -R nereus:nereus /var/lib/nereus /var/log/nereus && \
    chmod 755 /etc/nereus && \
    chmod 750 /var/lib/nereus /var/log/nereus

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/nereus .

# Copy sample configuration
COPY --chown=nereus:nereus examples/config.yaml /etc/nereus/config.yaml

# Copy MIB files if they exist
COPY --chown=nereus:nereus mibs/ /usr/share/snmp/mibs/

# Set proper permissions
RUN chmod +x ./nereus

# Switch to non-root user
USER nereus

# Expose ports
# 162/udp - SNMP trap listener
# 9090/tcp - Metrics and health endpoints
EXPOSE 162/udp 9090/tcp

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:9090/health || exit 1

# Set environment variables
ENV NEREUS_CONFIG=/etc/nereus/config.yaml
ENV NEREUS_LOG_LEVEL=info

# Volume for persistent data
VOLUME ["/var/lib/nereus", "/var/log/nereus"]

# Labels for metadata
LABEL maintainer="GeekxFlood <contact@geekxflood.com>" \
      org.opencontainers.image.title="Nereus SNMP Trap Alerting System" \
      org.opencontainers.image.description="Advanced SNMP trap alerting system with intelligent event correlation" \
      org.opencontainers.image.url="https://github.com/geekxflood/nereus" \
      org.opencontainers.image.source="https://github.com/geekxflood/nereus" \
      org.opencontainers.image.vendor="GeekxFlood" \
      org.opencontainers.image.licenses="MIT"

# Default command
CMD ["./nereus", "--config", "/etc/nereus/config.yaml"]
