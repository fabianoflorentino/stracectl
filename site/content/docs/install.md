---
title: "Installation"
description: "How to install stracectl on Linux."
weight: 1
---

## Requirements

- **Linux** — uses `ptrace` via the `strace` binary
- **Go 1.26+** — for building from source or `go install`
- **strace** — must be installed on the host

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

## Container image

Pre-built images are published to Docker Hub on every release:

```bash
docker pull fabianoflorentino/stracectl:latest

# Pin to a specific version
docker pull fabianoflorentino/stracectl:v1.0.38
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
