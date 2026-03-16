---
title: "strace backend"
description: "Classic strace subprocess-based tracing backend (usage, requirements, troubleshooting)."
weight: 7
---

This page documents the classic `strace` backend used by `stracectl`.

## What it is

The `strace` backend spawns the system `strace` binary and parses its stderr
output into `SyscallEvent` values. It is the default fallback when eBPF is
unavailable or when explicitly selected via `--backend strace`. The tracer
requires the `strace` binary to be present in `PATH`.

## When it's selected

- `--backend auto` (default) selects eBPF when available; otherwise the CLI
  falls back to the `strace` backend.
- Use `--backend strace` to force the classic subprocess tracer.
- Use `--backend ebpf` to force eBPF when available.

## Runtime requirements

- `strace` executable available in `PATH` (install with `apt install strace` on Debian/Ubuntu).
- Attaching to arbitrary running processes requires `CAP_SYS_PTRACE` (typically root)
  or kernel YAMA ptrace scope set to 0 — check your distribution docs.
- Tracing a child process started by `strace` generally does not require extra privileges.

## How it runs

`stracectl` launches `strace` with these flags by default: `-f -T -q`. The tracer
reads `strace`'s stderr stream, parses lines and emits `SyscallEvent` values used
by the aggregator, TUI and HTTP API.

Examples:

```bash
# Trace a program using the strace backend
stracectl run --backend strace curl https://example.com

# Attach to a running process (requires ptrace permissions)
sudo stracectl attach --backend strace 1234

# Enable verbose parser diagnostics (gates raw strace diagnostics)
stracectl run --backend strace --debug curl https://example.com
```

Notes:

- When tracing a launched program, `stracectl` puts `strace` and its traced
  children into a separate process group so that a single cancellation kills
  the whole group (avoids orphaned children).
- When attaching, `strace` is invoked with `-p PID`.

## Troubleshooting

- Error: `strace not found in PATH — install it with: apt install strace`
  - Install the `strace` package or ensure it's in `PATH`.
- Permission denied attaching to a PID
  - Check `CAP_SYS_PTRACE` or kernel YAMA `ptrace_scope` settings; run as root or adjust kernel settings.
- Parser parse errors or missing fields
  - The parser handles interleaved `<unfinished ...>` and `<... resumed>`
    lines by buffering prefixes per PID. Some edge cases can produce parse warnings; enable
    `--debug` to see noisy diagnostics (e.g., `EAGAIN` with empty args).
- `strace:` lines (diagnostics from strace itself) are captured and logged if `strace` exits with an error.

## Implementation notes

- `internal/tracer/strace.go` — spawns `strace`, wires process group handling,
  reads stderr, and forwards parsed `SyscallEvent` values.
- `internal/parser/parser.go` — stateful `Parser` that stitches `<unfinished ...>`/`<... resumed>`
  pairs and extracts syscall name, args, return value and latency.
- `internal/models/event.go` — `SyscallEvent` data structure produced by the parser/tracers.
- The tracer increases the stderr scanner buffer to handle large syscall args (512KiB).
- Diagnostics:
  - The package-level `tracer.Debug` boolean enables additional noisy debug logs; the CLI sets this from the
    persistent `--debug` flag in `cmd/root.go`.

## See also

- `site/content/docs/ebpf.md` — eBPF backend (recommended for lower overhead when available).
- `docs/EBPF.md` — repository-level eBPF notes and build instructions.
- `cmd/root.go`, `internal/tracer/strace.go`, `internal/parser/parser.go`, `internal/models/event.go`
