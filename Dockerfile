# Builder stage
FROM golang:1.24.3-alpine3.20 AS builder

# Enable CGO and set platform
ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64

# Install dependencies for CGO & C++
RUN apk add --no-cache gcc g++ musl-dev

WORKDIR /app

# Copy go mod and sum
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with CGO
RUN go build -o main .

# Final smaller runtime stage
FROM alpine:3.22

# Install runtime dependencies: C++ standard libs and SSL certs
RUN apk add --no-cache libstdc++ libgcc ca-certificates

WORKDIR /app

# Copy built binary from builder
COPY --from=builder /app/main .

# Expose port
EXPOSE 8080

# Start app
CMD ["./main"]
