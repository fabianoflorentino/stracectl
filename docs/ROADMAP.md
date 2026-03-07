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

### Fix `<unfinished ...>` line merging in the parser

**Goal:** preserve latency data for blocking syscalls in multi-threaded processes.

**Why:** when `strace -f` traces multiple threads, a syscall that blocks is split across two lines:

```shell
[pid 42] read(5,  <unfinished ...>
[pid 42] <... read resumed> "hello", 5) = 5 <0.002314>
```

Currently the first line is discarded; the second line is reconstructed but loses the original arguments. This means `Count` is accurate but `Latency` data is captured (latency comes from the `resumed` line, so it is actually preserved) while `Args` from the first line are lost.

**Approach:**

- Add a `pendingLines map[int]string` keyed by PID to `Parser` (make `Parse` a method on a stateful struct, or pass the map as a parameter)
- On `<unfinished ...>`: store the partial line in `pendingLines[pid]`; return `nil, nil`
- On `<... resumed>`: look up `pendingLines[pid]`, splice prefix from stored line + suffix from resumed line, delete the entry, then continue with normal parsing
- Update `parser_test.go` with multi-thread fixture cases

**Files:** `internal/parser/parser.go`, `internal/parser/parser_test.go`

---

### Flag `--container` on `attach` to auto-discover PID

**Goal:** remove the need to manually compose `stracectl discover | stracectl attach` inside a Pod.

**Why:** the sidecar manifest currently uses `"1"` as a placeholder PID, which requires a manual step or init-container workaround before tracing starts.

**Approach:**

- Add `--container <name>` flag to `cmd/attach.go`
- When set, call `discover.FindPID(name, "/proc")` internally before attaching
- If no matching process is found, return a clear error
- Update the sidecar manifest to use `--container app` instead of a hardcoded PID

**Files:** `cmd/attach.go`, `deploy/k8s/sidecar-pod.yaml`, `deploy/helm/stracectl/values.yaml`, `deploy/helm/stracectl/templates/_helpers.tpl`

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

### Expose `MinTime` in the API and TUI

**Goal:** surface the minimum observed latency per syscall, which is already tracked by the aggregator.

**Why:** `SyscallStat.MinTime` is computed but never returned by the JSON API or shown in the TUI. A very low `MinTime` alongside a high `MaxTime` indicates latency spikes rather than consistent slowness.

**Approach:**

- Verify `SyscallStat` JSON tags — if `MinTime` already serialises (no `json:"-"`), the API change is a no-op
- Add a `MIN` column to the TUI between `AVG` and `MAX`, with sort key `m`
- Add `SortByMin` to the `Sorted()` sort fields in the aggregator

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

### `stracectl stats <file>` — post-mortem analysis

**Goal:** analyse a raw `strace -o <file>` output file offline.

**Approach:**

- Add a `stats` subcommand that reads a file line by line through the existing `parser.Parse()` pipeline and feeds the aggregator
- Display the same TUI or, with `--serve`, the same HTTP API
- No tracer involved — just `os.Open` + `bufio.Scanner`

**Files:** `cmd/stats.go` (new), wired into `cmd/root.go`

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

## Hardening (sidecar security posture)

Even before the eBPF backend lands, the sidecar manifest can be tightened:

```yaml
securityContext:
  runAsUser: 0
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop: [ALL]
    add: [SYS_PTRACE]
```

This limits the blast radius compared to the current configuration while keeping `ptrace` functional.

**Files:** `deploy/k8s/sidecar-pod.yaml`, `deploy/helm/stracectl/values.yaml`
