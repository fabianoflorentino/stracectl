---
title: "eBPF backend"
description: "Overview, build and runtime requirements for the optional eBPF tracer."
weight: 6
---

This page documents the optional eBPF tracing backend available in `stracectl`.

## What it is

The eBPF backend attaches a small BPF program to kernel tracepoints and
emits syscall events via a ringbuffer. It avoids spawning a `strace`
subprocess and generally has lower overhead.

## When it's selected

- `--backend auto` (default) will choose the eBPF backend when the runtime
  kernel supports the required features (Linux >= 5.8) and the binary was
  compiled with eBPF support.
- Use `--backend ebpf` to force eBPF when you know the environment is
  compatible. Use `--backend strace` to force the classic subprocess tracer.

## Runtime requirements

- Linux kernel >= 5.8 (BPF ringbuf support).
- Privileges to load eBPF programs (typically root or equivalent capabilities).

## How to build an eBPF-enabled binary

Building locally requires `clang`, kernel headers and `bpf2go`:

```bash
# Install bpf2go
go install github.com/cilium/ebpf/cmd/bpf2go@latest

# Generate and build
go generate ./internal/tracer/...
CGO_ENABLED=1 go build -tags=ebpf -o stracectl-ebpf ./...
```

Alternatively, the project `Dockerfile` exposes a `production-ebpf` target that
produces a static, eBPF-enabled binary:

```bash
docker build --target production-ebpf -t stracectl:ebpf .
```

## Troubleshooting

- If eBPF load fails, check kernel version and privileges.
- Fall back to `--backend strace` if the environment cannot support eBPF.

## See also

- `internal/tracer/bpf/syscall.c` — the embedded BPF program source
- `docs/EBPF.md` — repository-level notes on building and runtime requirements
