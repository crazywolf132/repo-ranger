# Build Stage
FROM golang:1.20-alpine AS builder
WORKDIR /app

# Install Git (required for diff extraction)
RUN apk add --no-cache git

# Cache Go module dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code and build the binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o repo-ranger .

# Final Stage: Use a minimal image
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/repo-ranger .
ENTRYPOINT ["/app/repo-ranger"]
