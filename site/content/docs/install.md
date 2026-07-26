---
title: "Installation"
description: "How to install stracectl on Linux."
weight: 1
---

## Requirements

- **Linux** — uses `ptrace` via the `strace` binary, or the optional eBPF backend
- **Go 1.26+** — for building from source or `go install`
- **strace** — must be installed on the host (required for the `strace` backend)

### eBPF backend (optional)

`stracectl` also supports an eBPF tracing backend that avoids spawning a `strace`
subprocess and has lower overhead. Requirements for eBPF:

- Linux kernel >= 5.8 (BPF ringbuf support)
- `clang`, `llvm`, and kernel headers installed
- `bpf2go` tool: `go install github.com/cilium/ebpf/cmd/bpf2go@latest`

See the [eBPF backend](/docs/ebpf/) page for build instructions and details.

Install `strace` first:

```bash
# Debian / Ubuntu
sudo apt install strace

# Fedora / RHEL
sudo dnf install strace

# Alpine
apk add strace
```

## Install via `go install`

The quickest way to get the latest release:

```bash
go install github.com/fabianoflorentino/stracectl@latest
```

The binary will land in `$GOPATH/bin` (or `~/go/bin`). Make sure that directory is in your `$PATH`.

## Build from source

```bash
git clone https://github.com/fabianoflorentino/stracectl
cd stracectl
go build -o stracectl .
sudo mv stracectl /usr/local/bin/
```

Build with eBPF support (requires `clang` and `bpf2go`):

```bash
go generate ./internal/tracer/...
CGO_ENABLED=1 go build -tags=ebpf -o stracectl .
```

## Container image

Pre-built images are published to Docker Hub on every release:

```bash
docker pull fabianoflorentino/stracectl:latest

# Pin to a specific version
docker pull fabianoflorentino/stracectl:v1.0.131
```

Run inside a privileged container (required for `ptrace`):

```bash
docker run --rm --privileged \
  fabianoflorentino/stracectl:latest \
  run curl https://example.com
```

## Permissions

`stracectl` calls `strace` under the hood, which requires `CAP_SYS_PTRACE`.

```bash
# Option 1 — run with sudo (recommended for local use)
sudo stracectl run nginx

# Option 2 — allow ptrace for your user (less restrictive, not for production)
echo 0 | sudo tee /proc/sys/kernel/yama/ptrace_scope
```

> In Kubernetes sidecar pods the `securityContext` in the Helm chart and raw manifests
> already grants the necessary capabilities.

## Troubleshooting

When diagnosing parsing or tracer issues, run `stracectl` with the `--debug` flag
to enable verbose tracer diagnostics. This emits additional raw-strace lines that can
help identify parser edge cases such as `EAGAIN` with empty argument lists.

Example (local):

```bash
sudo stracectl --debug run --serve :8080 curl https://example.com
```

When running inside a container or sidecar, pass `--debug` in the container's
command/args. Only enable `--debug` during troubleshooting — it's intentionally
noisy and not suitable for long-running production usage.
