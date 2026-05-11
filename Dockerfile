# ---- build stage ----
FROM golang:1.22-alpine AS builder

WORKDIR /src

# Cache dependency downloads separately from source
COPY go.mod go.sum ./
RUN go mod download

# Copy full source
COPY . .

# Compile a fully-static binary (no CGO needed — pgx is pure Go)
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w" \
      -o /out/api \
      ./cmd/api

# ---- final stage ----
FROM alpine:3.20

# ca-certificates is required for TLS calls to Google's OAuth userinfo endpoint
RUN apk add --no-cache ca-certificates wget

# Run as a non-root user
RUN addgroup -S app && adduser -S -G app app

WORKDIR /app

# Static binary
COPY --from=builder /out/api ./api

# Migration SQL files — the runner reads these at startup
COPY --chown=app:app migrations/ ./migrations/

USER app

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/api"]
