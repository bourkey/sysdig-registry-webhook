# Multi-stage Dockerfile for Registry Webhook Scanner
# Stage 1: Build the Go application
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o scanner-webhook \
    ./cmd/scanner-webhook

# Stage 2: Download Sysdig CLI Scanner
FROM alpine:3.19 AS sysdig-downloader

# Install curl and verify tools
RUN apk add --no-cache curl

# Download Sysdig CLI Scanner
# Version pinning strategy: Update this version for new releases
ENV SYSDIG_CLI_VERSION=1.8.0

RUN curl -LO "https://download.sysdig.com/scanning/bin/sysdig-cli-scanner/${SYSDIG_CLI_VERSION}/linux/amd64/sysdig-cli-scanner" && \
    chmod +x sysdig-cli-scanner

# Stage 3: Final minimal image
FROM alpine:3.19

# Install CA certificates for HTTPS connections
RUN apk add --no-cache ca-certificates tzdata && \
    update-ca-certificates

# Create non-root user
RUN addgroup -g 1000 scanner && \
    adduser -D -u 1000 -G scanner scanner

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/scanner-webhook /app/scanner-webhook

# Copy Sysdig CLI Scanner
COPY --from=sysdig-downloader /sysdig-cli-scanner /usr/local/bin/sysdig-cli-scanner

# Create directories for config and secrets
RUN mkdir -p /app/config /app/secrets && \
    chown -R scanner:scanner /app

# Switch to non-root user
USER scanner

# Expose HTTP port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Set default environment variables
ENV PORT=8080 \
    LOG_LEVEL=info \
    CONFIG_FILE=/app/config/config.yaml \
    SYSDIG_CLI_PATH=/usr/local/bin/sysdig-cli-scanner

# Run the application
ENTRYPOINT ["/app/scanner-webhook"]
