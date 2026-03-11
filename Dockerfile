# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build binary
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o homedash ./cmd/homedash/

# Runtime stage
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary and default assets
COPY --from=builder /build/homedash .
COPY --from=builder /build/web ./web
COPY --from=builder /build/integrations ./integrations

# Default config (can be overridden via volume mount)
COPY --from=builder /build/config.yaml ./config.yaml

EXPOSE 8080

ENTRYPOINT ["./homedash"]
