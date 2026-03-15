# Unified multi-stage Dockerfile supporting:
#  - non-eBPF production build (default)
#  - eBPF-enabled production build (target: production-ebpf)
#  - development image with live reload and eBPF tooling
#  - Hugo site build/serve
#
# Build examples:
#  # Non-eBPF production (default):
#  docker build --target production -t stracectl:latest .
#
#  # eBPF-enabled production (builds eBPF artifacts, may require clang/linux-headers):
#  docker build --target production-ebpf -t stracectl:ebpf .
#
#  # Development with live-reload (includes strace, bpf2go):
#  docker build --target development -t stracectl:dev .

# -----------------------------------------------------------------------------
# Stage 0: bpf-build — compile BPF C into an object with clang (optional)
# -----------------------------------------------------------------------------
FROM debian:bookworm-slim AS bpf-build

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
       clang \
       llvm \
       linux-headers-amd64 \
       libbpf-dev \
       make \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /bpf

# Copy only BPF sources and compile to syscall.o for use by the ebpf build.
COPY internal/tracer/bpf/ ./

RUN clang -O2 -g -target bpf \
      -D__TARGET_ARCH_x86 \
      -I/usr/include/x86_64-linux-gnu \
      -c syscall.c \
      -o syscall.o

# -----------------------------------------------------------------------------
# Stage 1: base — lightweight non-eBPF Go build (alpine)
# -----------------------------------------------------------------------------
FROM golang:alpine3.23 AS base

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Remove bpf2go-generated Go and object files so non-eBPF builds don't
# fail due to missing embedded .o files. These files are required only for
# eBPF builds and will remain available in the `base-ebpf` stage.
RUN rm -f internal/tracer/ebpf_bpfel.go internal/tracer/ebpf_bpfeb.go \
  internal/tracer/ebpf_bpfel.o internal/tracer/ebpf_bpfeb.o || true

# Build the non-eBPF binary (CGO disabled for portability)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags="-s -w" -o /usr/local/bin/stracectl-non-ebpf .

# -----------------------------------------------------------------------------
# Stage 2: base-ebpf — full Go build with eBPF tag (requires clang/headers)
# -----------------------------------------------------------------------------
FROM golang:1.26-bookworm AS base-ebpf

WORKDIR /app

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
       clang \
       llvm \
       linux-headers-amd64 \
       libbpf-dev \
  && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy the compiled BPF object from the bpf-build stage (if present).
COPY --from=bpf-build /bpf/syscall.o ./internal/tracer/bpf/

# Build the eBPF-enabled binary (static linking as in project Dockerfile.eBPF).
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
  go build -trimpath -tags=ebpf -ldflags="-s -w -extldflags '-static'" -o /usr/local/bin/stracectl-ebpf .

# -----------------------------------------------------------------------------
# Stage 3: development — SDK image with live-reload, strace and eBPF tooling
# -----------------------------------------------------------------------------
FROM golang:1.26-bookworm AS development

WORKDIR /app

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
       strace \
       clang \
       llvm \
       linux-headers-amd64 \
       libbpf-dev \
       wget \
       git \
  && rm -rf /var/lib/apt/lists/* \
  && go install github.com/air-verse/air@latest \
  && go install github.com/cilium/ebpf/cmd/bpf2go@latest

EXPOSE 8080

ENTRYPOINT ["/go/bin/air"]
CMD ["-c", "/app/.air.toml"]

# -----------------------------------------------------------------------------
# Stage 4: site — Hugo development server for site previews
# -----------------------------------------------------------------------------
FROM alpine:3.23 AS site

ARG HUGO_VERSION=0.157.0
ARG HUGO_URL="https://github.com/gohugoio/hugo/releases"
ARG HUGO_PKG="hugo_extended_${HUGO_VERSION}_linux-amd64.tar.gz"

RUN apk add --no-cache git libc6-compat libstdc++ \
  && wget -q -O /tmp/hugo.tar.gz \
       "${HUGO_URL}/download/v${HUGO_VERSION}/${HUGO_PKG}" \
  && tar -xzf /tmp/hugo.tar.gz -C /usr/local/bin hugo \
  && rm /tmp/hugo.tar.gz \
  && hugo version

WORKDIR /site

EXPOSE 1313

ENTRYPOINT ["hugo"]
CMD ["server", "--bind", "0.0.0.0", "--disableFastRender", "--buildDrafts"]

# -----------------------------------------------------------------------------
# Stage 5: strace-src — provides a glibc-linked strace binary used by production
# -----------------------------------------------------------------------------
FROM debian:bookworm-slim AS strace-src

RUN apt-get update \
  && apt-get install -y --no-install-recommends strace \
  && rm -rf /var/lib/apt/lists/*

# -----------------------------------------------------------------------------
# Stage 6: production (non-eBPF, uses the lightweight non-ebpf binary)
# -----------------------------------------------------------------------------
FROM gcr.io/distroless/base:nonroot AS production

COPY --from=strace-src /usr/bin/strace /usr/bin/strace
COPY --from=base /usr/local/bin/stracectl-non-ebpf /usr/local/bin/stracectl

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/stracectl"]
CMD ["--help"]

# -----------------------------------------------------------------------------
# Stage 7: production-ebpf (eBPF-enabled production binary; larger/static)
# -----------------------------------------------------------------------------
FROM gcr.io/distroless/static:nonroot AS production-ebpf

COPY --from=strace-src /usr/bin/strace /usr/bin/strace
COPY --from=base-ebpf /usr/local/bin/stracectl-ebpf /usr/local/bin/stracectl

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/stracectl"]
CMD ["--help"]
