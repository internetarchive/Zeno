# Builder stage
FROM golang:1.24.3-alpine3.20 AS builder

WORKDIR /app

# Copy go mod and sum
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build
RUN go build -o main .

# Final smaller runtime stage
FROM alpine:3.22

# Install runtime dependencies: SSL certs
RUN apk add --no-cache ca-certificates

WORKDIR /app

# Copy built binary from builder
COPY --from=builder /app/main .

# Expose port
EXPOSE 8080

# Start app
CMD ["./main"]
