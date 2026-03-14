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

New flags and changes
---------------------

This change introduces new CLI flags and tracer configuration options to
give operators more control over eBPF behaviour and to aid troubleshooting
on platforms with non-standard kernels (for example, some WSL builds).

- `--force-ebpf`: when set, the eBPF probe will return an error instead of
  falling back to the classic `strace` tracer. Useful for debugging or for
  environments where you want to ensure eBPF is used exclusively.
- `--unfiltered`: disables the PGID filter written into the BPF `root_pgid`
  map so the BPF program captures events system-wide. This is useful when
  the tracer cannot reliably read task->pgid from kernel memory (a symptom
  seen in some WSL kernels). Use with caution: unfiltered mode captures
  events from all processes and may generate a high volume of events.
- `--try-elevate`: already present on the CLI, this flag instructs `stracectl`
  to attempt re-running the current process with `prlimit --memlock=unlimited`
  (prefixed with `sudo` when not root) if loading eBPF objects fails due to
  RLIMIT_MEMLOCK limits. This automates a common remediation step when
  running eBPF-enabled binaries in interactive shells.

Examples
--------

Capture system-wide events with the eBPF backend (useful on WSL when the
PGID filter drops events):

```bash
sudo stracectl attach --backend ebpf --unfiltered 1234
```

Run a command and ask the CLI to automatically re-exec with increased
memlock if needed:

```bash
stracectl run --try-elevate curl https://example.com
```

Implementation notes
--------------------

- The `EBPFTracer` gained two configuration setters: `SetForce(bool)` and
  `SetUnfiltered(bool)`. These are available in both `ebpf` and non-`ebpf`
  builds (the non-ebpf stub provides no-op setters) so the CLI can always
  configure the tracer without depending on build tags.
- The CLI now applies eBPF options via a small helper, `applyEBPFOptions`, so
  the `attach` and `run` commands can configure the tracer in a build-tag
  safe way.
- Unit tests were added to validate the wiring between CLI flags and the
  tracer configuration.

If you want these changes documented elsewhere (README, site, or a
CHANGELOG entry), tell me where to add them and I will create a separate
commit for each location.
