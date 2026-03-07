# ── Stage 1: build ────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

WORKDIR /src

# Cache module downloads before copying source
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /stracectl .

# ── Stage 2: runtime ──────────────────────────────────────────────────────────
FROM alpine:3.21

# strace is required at runtime; curl is useful for health probes
RUN apk add --no-cache strace

COPY --from=builder /stracectl /usr/local/bin/stracectl

# Default: expose the HTTP sidecar API on port 8080
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/stracectl"]
CMD ["--help"]
