# Multi-stage Dockerfile — single file for all environments.
#
# Stages / targets:
#   base          — downloads modules and compiles the binary (internal)
#   development   — standalone Go SDK image with air and strace for live-reload
#   strace-src    — provides the glibc-linked strace binary (internal)
#   production    — distroless runtime with strace; runs as nonroot (default)
#
# Build examples:
#   # Development (via docker compose):
#   docker compose up
#
#   # Development (standalone):
#   docker build --target development -t stracectl:dev .
#
#   # Production (default):
#   docker build -t stracectl:latest .
#   docker build --target production -t stracectl:latest .

# ── Stage 1: base ─────────────────────────────────────────────────────────────
FROM golang:alpine3.23 AS base

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o /usr/local/bin/stracectl .

# ── Stage 2: development ──────────────────────────────────────────────────────
# Starts fresh from the Go SDK — does NOT inherit the build artifacts from base.
# Source code and module/build caches are bind-mounted at runtime via docker compose,
# so they are intentionally excluded from the image.
FROM golang:alpine3.23 AS development

WORKDIR /app

# strace: required at runtime
# wget:   sample workload used by .air.toml
# git:    required by some go tooling
RUN apk add --no-cache strace wget git \
  && go install github.com/air-verse/air@latest

EXPOSE 8080

ENTRYPOINT ["/go/bin/air"]
CMD ["-c", "/app/.air.toml"]

# ── Stage 3: strace-src ───────────────────────────────────────────────────────
# Provides a glibc-linked strace binary for the production stage.
# distroless/base includes glibc, so the binary runs correctly.
FROM debian:bookworm-slim AS strace-src

RUN apt-get update \
  && apt-get install -y --no-install-recommends strace \
  && rm -rf /var/lib/apt/lists/*

# ── Stage 4: production ───────────────────────────────────────────────────────
# distroless/base (not static) is required because strace is glibc-linked.
FROM gcr.io/distroless/base:nonroot AS production

COPY --from=strace-src /usr/bin/strace /usr/bin/strace
COPY --from=base /usr/local/bin/stracectl /usr/local/bin/stracectl

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/stracectl"]
CMD ["--help"]
