# syntax=docker/dockerfile:1

# ---- Builder stage ----
FROM golang:1.23-alpine AS builder

WORKDIR /src

# Download dependencies first so this layer is cached unless go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and build a fully static binary (CGO disabled).
# -p=1 limits compiler parallelism to keep peak memory low on small build hosts.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -p=1 -trimpath -ldflags="-s -w" \
    -o /out/relay ./cmd/relay

# ---- Final stage ----
# Use alpine (not distroless) for the runtime: it is still tiny (~8MB) but ships a
# shell and BusyBox wget, which lets the HEALTHCHECK probe /healthz without baking
# extra tooling into the image. ca-certificates is added for outbound HTTPS to
# upstream LLM/TTS APIs.
FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget tini \
    && addgroup -S relay \
    && adduser -S -G relay -u 10001 relay

WORKDIR /app

# Binary and default config.
COPY --from=builder /out/relay /app/relay
COPY --from=builder /src/configs/config.yaml /app/configs/config.yaml

# Writable logs directory owned by the non-root run user.
RUN mkdir -p /app/logs && chown -R relay:relay /app

EXPOSE 8080

# Probe the relay's /healthz endpoint with BusyBox wget.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --spider http://127.0.0.1:8080/healthz || exit 1

# Run as a non-root user.
USER relay

# tini is PID 1 and forwards SIGINT/SIGTERM to the relay so graceful shutdown works;
# exec form keeps the binary as a direct child (no shell to swallow signals).
ENTRYPOINT ["/sbin/tini", "--", "/app/relay"]
CMD ["-config", "/app/configs/config.yaml"]
