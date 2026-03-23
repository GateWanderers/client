# ── Stage 1: Build ──────────────────────────────────────────────────────────
FROM golang:1.23-bookworm AS builder

WORKDIR /build

# Download dependencies first (cached layer).
COPY server/go.mod server/go.sum ./
RUN go mod download

# Copy source and build.
COPY server/ ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /gatewanderers ./cmd/server

# ── Stage 2: Runtime ─────────────────────────────────────────────────────────
FROM debian:bookworm-slim AS runtime

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Binary from builder.
COPY --from=builder /gatewanderers ./gatewanderers

# Static web assets.
COPY server/web/ ./web/

# Non-root user for runtime security.
RUN useradd -r -u 1001 -g root gw
USER 1001

EXPOSE 8080

ENTRYPOINT ["./gatewanderers"]
