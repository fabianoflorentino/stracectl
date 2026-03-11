# stracectl — Implementation Roadmap

This document tracks planned features, known technical debt, and the implementation notes needed to address each item.

---

## Recent: Debug flag and tracing diagnostics

- **Status:** partially implemented
- **What changed:** added a global `--debug` CLI flag (registered in `cmd/root.go`) that gates verbose tracer diagnostics via `tracer.Debug`. When enabled, the tracer logs raw strace lines useful for diagnosing parser edge cases (for example, `EAGAIN` with empty `Args`). Noisy diagnostics are gated and only emitted when `--debug` is true.
- **Files touched:** `cmd/root.go`, `internal/tracer/strace.go`, `cmd/stats.go`, plus documentation updates in `README.md`, `docs/USAGE.md`, and the site under `site/content/docs/`.
- **Notes & next steps:**
  - The TUI currently discards standard logger output while in the alternate screen (`internal/ui/tui.go`), so `--debug` messages are not visible inside the TUI by default.
  - Planned: capture logger output when `--debug` is enabled and forward those messages into the aggregator's live-log buffer so they appear in the TUI log overlay (see `internal/aggregator/aggregator.go` and `internal/ui/tui.go`).
  - Planned: add optional file-based debug logging when `--debug` is set (useful for offline inspection).
  - Planned: de-emphasize `"<no data>"` in the UI and left-align timestamps in the logs/detail views.
  - Planned: add a test asserting `--ws-token` exists as a persistent flag and remove any duplicate flag definitions if present.

## Pending features

### Direct `ptrace` backend

**Goal:** remove the dependency on the `strace` binary by calling `ptrace(2)` directly from Go.

**Why:** eliminates the runtime requirement of having `strace` installed on the host; opens the door to structured event metadata that the text-based parser cannot provide.

**Approach:**

- Use `golang.org/x/sys/unix` to call `ptrace(PTRACE_ATTACH, pid, ...)` / `ptrace(PTRACE_SYSCALL, ...)` in a dedicated goroutine locked to its OS thread (`runtime.LockOSThread`)
- Read registers with `PTRACE_GETREGS` on `SIGTRAP` to capture syscall number and arguments
- Replace `tracer/strace.go` with a new `tracer/ptrace.go` implementing the same `<-chan models.SyscallEvent` interface — the rest of the pipeline is unchanged
- Keep the strace-subprocess tracer behind a `--backend strace` flag for fallback

**Files:** `internal/tracer/ptrace.go` (new), `internal/tracer/strace.go`, `cmd/attach.go`, `cmd/run.go`

---

### eBPF backend via `cilium/ebpf`

**Goal:** zero-overhead syscall tracing without `ptrace`, suitable for production environments.

**Why:** `ptrace` serialises the traced process on every syscall entry/exit (significant overhead for high-rate processes); eBPF tracepoints run in kernel context with negligible overhead.

**Approach:**

- Attach to `raw_tracepoint/sys_enter` and `raw_tracepoint/sys_exit` using `cilium/ebpf`
- Share a `BPF_MAP_TYPE_RINGBUF` ring buffer between the BPF program and user-space reader
- Each ring-buffer record encodes: PID, syscall number, return value, `bpf_ktime_get_ns()` timestamps for entry and exit — latency computed in user space
- Required capabilities: `CAP_BPF` + `CAP_PERFMON` (Linux 5.8+) — less privileged than `CAP_SYS_PTRACE`
- Implement the same `<-chan models.SyscallEvent` interface so the aggregator/server/TUI are unchanged
- Add `--backend ebpf` flag; make it the default when the kernel supports it

**Files:** `internal/tracer/ebpf.go` (new), `internal/tracer/bpf/syscall.c` (new), `cmd/attach.go`, `cmd/run.go`

---

### Per-file view

**Goal:** show which file paths are opened most often.

**Approach:**

- Parse the first argument of `openat` / `open` calls from `SyscallEvent.Args`
- Add a `FileStats` map to the aggregator
- Expose via `GET /api/files` and a new TUI tab toggled with `f`

**Files:** `internal/aggregator/aggregator.go`, `internal/server/server.go`, `internal/ui/tui.go`

---

### Per-socket view

**Goal:** show active connections, bytes sent, and bytes received.

**Approach:**

- Track `connect` / `accept` calls to build a connection table (fd → addr)
- Accumulate byte counts from `sendto` / `recvfrom` return values
- Expose via `GET /api/sockets` and a TUI tab toggled with `s`

**Files:** `internal/aggregator/aggregator.go`, `internal/server/server.go`, `internal/ui/tui.go`

---

### Flamegraph-style syscall timeline

**Goal:** visualise the temporal distribution of syscalls to spot bursts and idle gaps.

**Approach:**

- Buffer `SyscallEvent` timestamps in a fixed-size ring (e.g., last 10 s at 10 ms resolution)
- Render as a sparkline in the TUI footer
- Expose raw time-series via `GET /api/timeline`

**Files:** `internal/aggregator/aggregator.go`, `internal/server/server.go`, `internal/ui/tui.go`

---

### Process tree view for multi-process tracing (`-f`)

**Goal:** group syscall stats by process/thread when tracing with `-f`.

**Approach:**

- Key the aggregator by `(PID, syscall)` instead of just `syscall`
- Add a `--per-pid` flag to `run` and `attach`
- TUI shows a collapsible tree: parent PID → children → syscalls
- API adds `pid` field to each `SyscallStat` entry

**Files:** `internal/aggregator/aggregator.go`, `internal/ui/tui.go`, `internal/server/server.go`, `cmd/attach.go`, `cmd/run.go`

---

### HTML report — anomaly section and timeline sparkline

**Goal:** extend the existing `--report` HTML export with the remaining planned sections.

**Current state:** the report already includes a header, summary bar, category breakdown, and a sortable syscall table. The following sections are not yet implemented.

**Remaining work:**

- **Anomaly section** — list the same alerts that appear in the TUI banner, each with a human-readable explanation
- **Timeline sparkline** — per-second call rate rendered as an inline SVG path (requires the ring-buffer timeline feature below)

**Files:** `internal/report/report.go`, `internal/report/static/report.html`
