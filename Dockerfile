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
    bpftool \
    dwarves \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /bpf

# Copy only BPF sources and compile to syscall.o for use by the ebpf build.
COPY internal/tracer/bpf/ ./

# Attempt to dump kernel BTF into a vmlinux.h in the build context so the
# subsequent clang invocation can include it. This will succeed when the
# build environment exposes kernel BTF (most hosts expose /sys/kernel/btf).
RUN if [ -f /sys/kernel/btf/vmlinux ]; then \
      echo "Dumping /sys/kernel/btf/vmlinux -> /bpf/vmlinux.h" && \
      bpftool btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h || true; \
    elif [ -f /boot/vmlinux-$(uname -r) ]; then \
      echo "Dumping /boot/vmlinux-$(uname -r) -> /bpf/vmlinux.h" && \
      bpftool btf dump file /boot/vmlinux-$(uname -r) format c > vmlinux.h || true; \
    else \
      echo "No kernel BTF found in build environment; continuing without vmlinux.h"; \
    fi && ls -la /bpf || true

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
  # Install bpftool and dwarves (pahole) so we can dump BTF to vmlinux.h when available
RUN apt-get update \
  && apt-get install -y --no-install-recommends \
       bpftool \
       dwarves \
  && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy the compiled BPF object and sources from the bpf-build stage (if present).
# Copying the whole `/bpf/` ensures `syscall.c` is available for bpf2go.
COPY --from=bpf-build /bpf/ ./internal/tracer/bpf/

# Ensure `bpf2go` is available (best-effort), attempt to dump vmlinux BTF
# into `internal/tracer/bpf/vmlinux.h` (if builder exposes it), then
# generate BPF artifacts so the `-tags=ebpf` build has the required
# generated Go wrappers and .o files embedded.
RUN go install github.com/cilium/ebpf/cmd/bpf2go@latest || true && \
    export PATH="$(go env GOPATH)/bin:$PATH" && \
    if [ -f /sys/kernel/btf/vmlinux ]; then \
      echo "Found /sys/kernel/btf/vmlinux - dumping to internal/tracer/bpf/vmlinux.h" && \
      bpftool btf dump file /sys/kernel/btf/vmlinux format c > internal/tracer/bpf/vmlinux.h || true ; \
    elif [ -f /boot/vmlinux-$(uname -r) ]; then \
      echo "Found /boot/vmlinux-$(uname -r) - dumping to internal/tracer/bpf/vmlinux.h" && \
      bpftool btf dump file /boot/vmlinux-$(uname -r) format c > internal/tracer/bpf/vmlinux.h || true ; \
    else \
      echo "No kernel BTF found in build environment; skipping vmlinux.h generation" ; \
    fi && \
    bash scripts/generate-bpf.sh

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
  && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/* \
  && go install github.com/air-verse/air@latest \
  && go install github.com/cilium/ebpf/cmd/bpf2go@latest \
  && go clean -modcache \
  && rm -rf /root/.cache/go-build "$(go env GOPATH)"/pkg/mod

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
# Stage 6: production — single image containing both non-eBPF and eBPF binaries
# -----------------------------------------------------------------------------
# Use distroless/cc (not distroless/static) because the eBPF binary is built
# with CGO_ENABLED=1 and requires glibc's dynamic linker
# (/lib64/ld-linux-x86-64.so.2). distroless/static ships without it and causes
# "exec: no such file or directory" at runtime.
FROM gcr.io/distroless/cc:nonroot AS production

# Copy a glibc-linked `strace` (as before) and both built binaries so a single
# image contains both backends. The non-eBPF binary will be the default
# `/usr/local/bin/stracectl`; the eBPF binary will also be present as
# `/usr/local/bin/stracectl-ebpf` for direct execution when needed.
COPY --from=strace-src /usr/bin/strace /usr/bin/strace
# Copy runtime shared libraries required by /usr/bin/strace.
# distroless/cc provides glibc but not libunwind-ptrace and related deps.
COPY --from=strace-src /usr/lib/x86_64-linux-gnu/libunwind.so* /usr/lib/x86_64-linux-gnu/
COPY --from=strace-src /usr/lib/x86_64-linux-gnu/libunwind-*.so* /usr/lib/x86_64-linux-gnu/
COPY --from=strace-src /usr/lib/x86_64-linux-gnu/liblzma.so* /usr/lib/x86_64-linux-gnu/
COPY --from=strace-src /lib/x86_64-linux-gnu/libgcc_s.so.1 /lib/x86_64-linux-gnu/
# Install a single `stracectl` binary in the image. Prefer the eBPF
# build (from `base-ebpf`) so the runtime image contains the fully-capable
# binary at `/usr/local/bin/stracectl`. The non-eBPF build is no longer
# separately installed in the final image.
COPY --from=base-ebpf /usr/local/bin/stracectl-ebpf /usr/local/bin/stracectl

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/stracectl"]
CMD ["--help"]
