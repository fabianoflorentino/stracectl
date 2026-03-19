# stracectl — Implementation Roadmap

This document tracks planned features, known technical debt, and the implementation notes needed to address each item.

---

## Pending features

### Per-file view

**Goal:** show which file paths are opened most often.

**Overview:** identify hot file paths from `open`/`openat` syscalls and surface
them in the TUI and sidecar API to help diagnose excessive I/O, repeated
ENOENTs, and misconfiguration.

**Implementation plan**

- Files to change:
  - `internal/aggregator/aggregator.go` — add `fileStats` storage and counting logic
  - `internal/server/server.go` — register `GET /api/files` and implement handler
  - `internal/ui/tui.go` — add files overlay toggled with `f`
  - `internal/aggregator/aggregator_test.go` — unit tests for parsing and counting
  - (optional) `internal/parser/parser.go` — move helper if desired

- Aggregator:
  - add `fileStats map[string]int64`, constants `fileStatsCap = 10000` and `maxPathLen = 1024`
  - initialize `fileStats` in `New()`
  - in `Add()`, for events `open` / `openat`, call `extractPathFromArgs(name,args)` to get the path
  - increment `fileStats[path]`, respecting the cardinality cap and truncation
  - expose `TopFiles(n int) []FileStat` returning sorted results

- Path extraction helper:
  - heuristic: prefer first quoted string; fallback to splitting by commas and selecting the right argument for `open` (first) and `openat` (second)
  - attempt `strconv.Unquote` to handle escaped sequences; return empty string when no plausible path found

- Server API:
  - register `s.registerRoute("/api/files", s.handleFiles, "Top opened files")`
  - handler supports optional `?limit=N` query param and returns JSON list of `{path,count}`

- TUI:
  - add `filesOverlay` state toggled by `f`, render `agg.TopFiles(limit)` in an overlay, support scrolling and truncation with tooltip

- Tests:
  - unit tests for `extractPathFromArgs` covering quoted, escaped, relative and missing args
  - aggregator unit test to verify counts and caps
  - optional integration test comparing `stracectl stats` output against a known `strace` capture

- Safety & limits:
  - cap distinct keys, truncate paths, and avoid dereferencing pointers (no ptrace peeks)
  - consider sampling or configurable cap for high-cardinality workloads

- User-visible results:
  - TUI `f` overlay showing most-opened file paths and counts
  - Sidecar `GET /api/files?limit=20` returning top files as JSON

- Next steps:
  1. Implement aggregator `fileStats` and `TopFiles()` (low-level).
  2. Add `/api/files` handler and route registration.
  3. Add TUI overlay and unit tests.
  4. (Optional) Extend HTML report to include top files.

**Estimated effort:** Aggregator + API: ~3–6 hours; TUI + tests: +0.5–1 day.

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

## Troubleshooting improvements

This section lists prioritized opportunities to improve the user's ability to diagnose
and recover from failures across the three operation modes: TUI (interactive),
Sidecar (HTTP), and Replay / Stats (offline).

### High-priority

- **Diagnose command (`stracectl diagnose`)**: run environment checks (is `strace` in PATH,
  kernel/eBPF compatibility, current `RLIMIT_MEMLOCK`, capabilities, effective UID) and print
  human-friendly suggestions plus machine-readable JSON output. Files: `cmd/diagnose.go`.
  Rationale: quick triage for common failures and automatable in CI.
- **`/api/diagnostics` endpoint**: expose current tracer state, selected backend, `ProcInfo`,
  aggregator totals, parse-failure and tracer-error counters, and recent raw tracer warnings.
  Files: `internal/server/server.go` and the dashboard JS. Rationale: immediate troubleshooting
  information in sidecar mode.
- **Parse & tracer Prometheus metrics**: add counters such as `stracectl_parse_failures_total`
  and `stracectl_tracer_errors_total` so failures are observable and alertable. Files:
  `internal/parser/parser.go`, `internal/tracer/*.go`, and `internal/server/server.go`.

### Medium-priority

- **TUI diagnostic overlay (key `D`)**: show backend, PID / `ProcInfo`, last raw tracer errors,
  parser failure counts and remediation hints (e.g. run `stracectl diagnose`, use `--try-elevate`).
  Files: `internal/ui/tui.go`, `internal/aggregator/aggregator.go`.
- **Replay flags**: add `--show-parse-errors`, `--strict`, and `--json` to the `stats` command to
  list skipped lines, optionally fail on parse errors, or output diagnostics JSON. Files: `cmd/stats.go`.
- **Improve error messages & remediation hints**: enrich eBPF/attach errors with actionable
  suggestions (for example the exact `prlimit` / `sudo` commands) and clearly explain fallbacks.
  Files: `internal/tracer/ebpf.go`, `cmd/run.go`, `cmd/attach.go`.

### Longer-term / Lower-priority

- **Structured debug logging and levels**: introduce `internal/log` with levels (info/warn/error/debug),
  optional runtime toggle via HTTP, and file capture when `--debug` is set. Forward selected logs into
  the aggregator live-log when requested for easier TUI inspection.
- **Dashboard diagnostics tab**: add a visual diagnostics tab to the web UI showing health checks,
  recent errors, and quick action hints (copy elevation command, open logs).
- **Persist debug traces**: optionally write raw tracer output to a file when `--debug` is enabled
  for offline analysis.

### Next steps

1. Implement high-priority items: `diagnose` command, `/api/diagnostics`, and Prometheus counters for
   parse/tracer failures.
2. Add TUI diagnostic overlay and the `stats` replay flags.
3. Follow up with structured logging and dashboard UI enhancements.

**Estimated effort:** High-priority: small (1–2 days). Medium: small→medium. Longer-term: medium→large.
