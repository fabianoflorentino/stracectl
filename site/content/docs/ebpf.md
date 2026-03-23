---
title: "eBPF backend"
description: "Overview, build and runtime requirements for the optional eBPF tracer."
weight: 8
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
CGO_ENABLED=1 go build -tags=ebpf -o stracectl ./...
```

Alternatively, the project `Dockerfile` provides a `production` target that
builds a single production image containing both the non-eBPF and eBPF
binaries. Build the image from the repository root:

```bash
docker build --target production -t stracectl:latest .
```

## Troubleshooting

- If eBPF load fails, check kernel version and privileges.
- Fall back to `--backend strace` if the environment cannot support eBPF.

## See also

- `internal/tracer/bpf/syscall.c` — the embedded BPF program source
- `docs/EBPF.md` — repository-level notes on building and runtime requirements
 
## New flags and changes

The project introduced several CLI flags and options to control eBPF behaviour:

- `--force-ebpf`: when set, the eBPF probe will return an error instead of falling back to the classic `strace` tracer. Useful for debugging or environments where eBPF must be enforced.
- `--unfiltered`: disables the PGID filter written into the BPF `root_pgid` map so the BPF program captures events system-wide. Use with caution: unfiltered mode can generate a high volume of events.
- `--try-elevate`: attempts to re-run the current process with increased `RLIMIT_MEMLOCK` (prefixed with `sudo` when not root) if loading eBPF objects fails due to memlock limits.

Examples:

```bash
# Force eBPF and fail if not available
sudo stracectl run --backend ebpf --force-ebpf curl https://example.com

# Capture system-wide events (useful in some WSL kernels)
sudo stracectl attach --backend ebpf --unfiltered 1234
```

Implementation notes:

- Non-ebpf builds include no-op setters so the CLI can always configure tracer options regardless of build tags.
- The CLI exposes helper functions to apply eBPF options safely across `run` and `attach` commands.
