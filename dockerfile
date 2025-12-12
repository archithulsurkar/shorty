# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install git for fetching dependencies
RUN apk add --no-cache git

# Copy all source files first
COPY . .

# Download dependencies and generate go.sum
RUN go mod tidy

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o shorty .

# Final stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Copy the binary from builder
COPY --from=builder /app/shorty .

# Expose port
EXPOSE 8080

# Run the application
CMD ["./shorty"]
