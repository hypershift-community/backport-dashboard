# Build stage
FROM golang:1.20-alpine AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o backport-dashboard .

# Runtime stage
FROM alpine:3.18

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/backport-dashboard .

# Copy UI files for the frontend
COPY ui/ /app/ui/

# Expose the application port
EXPOSE 8080

# Run the application
ENTRYPOINT ["/app/backport-dashboard"]
