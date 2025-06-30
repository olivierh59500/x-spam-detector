# Multi-stage build for X Spam Detector
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o spam-detector .

# Final stage - minimal image
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN adduser -D -s /bin/sh spamdetector

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/spam-detector .

# Copy configuration
COPY config.yaml .

# Create data and temp directories
RUN mkdir -p data temp && \
    chown -R spamdetector:spamdetector /app

# Switch to non-root user
USER spamdetector

# Expose port (if needed for web interface)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD ./spam-detector -health || exit 1

# Run the application
CMD ["./spam-detector"]