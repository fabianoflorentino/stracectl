# eBPF backend

This document briefly describes the optional eBPF tracing backend included in
`stracectl`: what it provides, runtime requirements, and how to build an
eBPF-enabled binary or container.

Overview
--------

The eBPF backend traces syscalls by attaching a small BPF program to raw
tracepoints (`sys_enter`/`sys_exit`) and publishing syscall events via a
ringbuffer. Compared with the classic `strace` subprocess tracer, eBPF offers
lower overhead and avoids spawning a separate `strace` process.

When to use
-----------

- Use the eBPF backend for lower tracing overhead and when you cannot or do
  not want to rely on `ptrace`/`strace`.
- Use `--backend ebpf` to force the eBPF backend, or `--backend auto` to let
  `stracectl` pick it when available.

Runtime requirements
--------------------

- Linux kernel >= 5.8 (BPF ringbuf support).
- Privileges to load eBPF programs (usually root, or appropriate capabilities such as `CAP_BPF`/`CAP_SYS_ADMIN` depending on kernel configuration).
- The binary must have been compiled with eBPF support (the `ebpf` build tag).

Build requirements (compile-time)
--------------------------------

To build a local eBPF-enabled binary you need:

- `clang`, `llvm`, and linux kernel headers installed so the BPF object can be built.
- `bpf2go` (from `github.com/cilium/ebpf/cmd/bpf2go`) to embed compiled BPF objects into the Go binary.

Local build example
-------------------

1. Install `bpf2go`:

```bash
go install github.com/cilium/ebpf/cmd/bpf2go@latest
```

1. Generate BPF artifacts and build with the `ebpf` tag (run from repo root):

```bash
go generate ./internal/tracer/...
CGO_ENABLED=1 go build -tags=ebpf -o stracectl-ebpf ./...
```

Docker build example
--------------------

The repository `Dockerfile` includes a `production-ebpf` target that builds
the BPF object and produces a static eBPF-enabled binary. From the project
root run:

```bash
docker build --target production-ebpf -t stracectl:ebpf .
```

Troubleshooting
---------------

- If `--backend ebpf` fails with a build/runtime error, try `--backend strace`.
- Ensure you have the correct kernel and privileges; loading BPF programs may
  fail with permission errors if not run as root or lacking necessary
  capabilities.
- For deep debugging, enable `--debug` to see additional tracer diagnostics.

See also
--------

- BPF program source: `internal/tracer/bpf/syscall.c`
- Site documentation: `site/content/docs/ebpf.md`
