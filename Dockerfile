# Multi-stage Dockerfile for Evaluer Go REST API Server

# -------------------------------------------------------------
# Stage 1: Build stage
# -------------------------------------------------------------
FROM golang:alpine AS builder

# Enable automatic Go toolchain fetching if needed
ENV GOTOOLCHAIN=auto

WORKDIR /app

# Install ca-certificates and git
RUN apk add --no-cache ca-certificates git

# Copy dependency manifests
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY tests/ ./tests/

# Build statically compiled binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/evaluer-server ./cmd/server

# -------------------------------------------------------------
# Stage 2: Minimal runtime stage
# -------------------------------------------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/evaluer-server /app/evaluer-server

# Default port
ENV PORT=8000
EXPOSE 8000

# Run the server binary
ENTRYPOINT ["/app/evaluer-server"]
