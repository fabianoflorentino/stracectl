# stracectl — Implementation Roadmap

This document tracks planned features, known technical debt, and the implementation notes needed to address each item.

---

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

### Optional WebSocket token authentication

**Goal:** prevent unauthenticated access to the `/stream` endpoint when the port is accidentally exposed outside the cluster.

**Why:** `CheckOrigin` currently returns `true` unconditionally; any client on any origin can connect.

**Approach:**

- Add `--ws-token <token>` flag to `server.New()` (and surface it in `cmd/attach.go` / `cmd/run.go`)
- In `handleStream`: before calling `upgrader.Upgrade()`, check for a `Bearer` token in the `Authorization` header or a `token` query parameter; return `401` if the token does not match
- When `--ws-token` is not set, keep the current open behaviour (backwards-compatible)

**Files:** `internal/server/server.go`, `cmd/attach.go`, `cmd/run.go`

---

### Expose `MinTime` in the main TUI table and bulk API

**Goal:** surface the minimum observed latency per syscall in the primary table view and bulk stats response.

**Current state:** `SyscallStat.MinTime` is computed by the aggregator, returned by `/api/syscall/{name}`, and shown in the TUI detail overlay (`d` key) and in the web detail page (`/syscall/{name}`). It is **not** shown as a column in the main TUI table and is not verified to be present in the `/api/stats` bulk response in a human-readable form (`time.Duration` serialises as integer nanoseconds).

**Remaining work:**

- Add a `MIN` column to the TUI table between `AVG` and `MAX`, with sort key `m`
- Add `SortByMin` to the `Sorted()` sort fields in the aggregator
- Decide whether `/api/stats` should format `MinTime` as a string or keep the raw nanosecond integer

**Files:** `internal/aggregator/aggregator.go`, `internal/ui/tui.go`

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
