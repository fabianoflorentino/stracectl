# stracectl — Implementation Roadmap

This document describes the project roadmap, prioritized work items, and concrete implementation notes. It highlights features that should be supported consistently across both tracing backends (the classic `strace` subprocess and the eBPF tracer) and lists short-, mid-, and long-term priorities.

> **Last updated:** 2026-07-26

---

## Already implemented

The following features are complete and shipped. They are listed here for reference only and should not be re-prioritized.

- **`--per-pid`** — group syscall rows by PID instead of aggregating across all processes.
- **`--report <path>`** — self-contained HTML report with sortable syscall table, category breakdown, top files.
- **`--report-top-files N`** — include top opened files in HTML reports.
- **`--serve <addr>`** — replace TUI with HTTP server (dashboard, WebSocket stream, Prometheus `/metrics`, JSON API).
- **`--ws-token`** — Bearer token authentication for WebSocket connections.
- **`--backend auto|ebpf|strace`** — backend selection with auto-detection.
- **`--container <name>`** — auto-discover container PID in Kubernetes pods.
- **Privacy pipeline** — `--privacy-log`, `--no-args`, `--max-arg-size`, `--redact-patterns`, `--privacy-level`, `--full`, `--force`, `--privacy-ttl`.
- **Syscall filtering (privacy)** — `--syscalls` and `--exclude` for privacy-scoped inclusion/exclusion.
- **`explain` command** — prints current privacy settings and shows a redacted example.
- **Anomaly alerts** — hardcoded patterns for common slow/error-prone syscalls (ioctl, connect, open, etc.).
- **P95/P99 percentiles** — log2 latency histogram with on-the-fly percentile computation.
- **Per-errno breakdown** — recent error samples and rolling 60s error rate.
- **FD mapper** — maps file descriptors to paths using `/proc/<pid>/fd` and heuristic extraction from open/openat args.
- **Top files** — TUI overlay (`f` key), API endpoint (`/api/files`), and HTML report integration.
- **Follow forks (`-f`)** — strace subprocess passes `-f`; eBPF uses PGID-based filtering with `--per-pid`.
- **Summary (`-c` style)** — aggregator produces calls/time/errors summary via `stats` command.

---

## Cross-backend compatibility: strace-like options we will support

Goal: provide a unified CLI and consistent operator experience regardless of backend. When a feature cannot be implemented identically on both backends we will provide a best-effort behavior and document the difference.

- **Save parsed events (`--save-events` → NDJSON/JSON)** — write parsed events to disk in a structured format for offline analysis.
  - Supported by both backends: format events as NDJSON and include fields `time`, `pid`, `name`, `args`, `retval`, `error`, `latency`, plus optional `decoded_fds`, `path`, `stack` when available.
  - Complexity: low.

- **Filter syscalls (`--filter-syscalls`)** — allow limiting traced syscalls or groups (file, net, process, memory) via a unified CLI flag.
  - strace: pass-through `-e trace=...`.
  - eBPF: implement a BPF map of enabled syscalls or apply a fast userspace filter after event emission.
  - Complexity: medium.

- **Path filtering (`--trace-path`)** — limit events to syscalls touching a path substring.
  - strace: pass-through `-P`.
  - eBPF: prefer matching on `Path` captured in the BPF event or filter in userspace; support writing path filters to a BPF map for kernel-side filtering later.
  - Complexity: medium.

- **String limit (`--string-limit`)** — control truncation of printed strings (read/write dumps).
  - strace: pass `-s N` to subprocess.
  - eBPF: truncate in userspace or recompile BPF with a larger buffer (requires BPF/C change).
  - Complexity: low.

- **Timestamps & precision (`--timestamps=relative|absolute|ns`)** — consistent timestamping across backends.
  - strace: parse timestamp prefixes when present.
  - eBPF: use `EnterNs`/`ExitNs` from BPF events and convert to wall-clock or relative time in userspace.
  - Complexity: medium.

- **Status filter (`--status=successful|failed`)** — show only errors or only successes.
  - Backend-agnostic: filter by `SyscallEvent.Error` in userspace.
  - Complexity: low.

- **FD decoding (`--decode-fds`)** — translate file descriptors into paths where possible.
  - strace: pass `-y` or implement userspace map of `/proc/<pid>/fd` lookups.
  - eBPF: prefer `Path` captured by BPF for common operations; fallback to resolving `/proc/<pid>/fd/<n>` in userspace.
  - The FD mapper (`internal/aggregator/fdmapper.go`) already exists; expose it as an opt-in CLI flag with caching.
  - Complexity: medium.

Notes:

- Where kernel-side BPF filtering is not yet possible, prefer efficient userspace filtering to avoid resource blowup.
- For any backend difference the CLI will print a short diagnostic explaining the fallback behavior (e.g., "eBPF: path filtering applied in userspace").

---

## Roadmap priorities

### Short-term (next 1–2 weeks)

- Add `--save-events <file>` (NDJSON) to `run`, `attach`, and `stats` so both backends can dump parsed events for offline analysis. Low risk, immediately useful.
- Expose `--filter-syscalls`, `--trace-path`, `--string-limit`, `--status` as unified CLI flags and wire them to `strace` (pass-through) and eBPF (userspace/BPF-map where practical).
- Add `--timestamps=relative|absolute|ns` option that controls how timestamps are computed and displayed for both backends.
- Expose `--decode-fds` as an opt-in flag that activates the existing fdmapper with `/proc/<pid>/fd` resolution and a basic cache.

### Mid-term (1–3 months)

- **Session save/load** — `stracectl save <session.sc>` and `stracectl load <session.sc>` to serialize/deserialize aggregator state as compressed JSON. Enables sharing, diffing, and post-mortem across sessions.
- **Session diff** — `stracectl diff <session1.sc> <session2.sc>` or a TUI overlay showing latency/count deltas between two saved sessions. Useful for before/after performance comparisons.
- **Configurable alert thresholds** — `--alert latency>50ms,errno=EACCES>10` to let users define custom alert rules instead of relying on hardcoded patterns.
- **Dynamic TUI filters** — filter by minimum latency (`--min-latency 10ms`), time range, or fd/path substring. Extend the existing TUI filter (`/` key) with these dimensions.
- **TopSockets** — surface the most-visited socket addresses, analogous to TopFiles. Server endpoint (`/api/sockets`) and TUI overlay (`s` key).
- **Timeline sparkline** — a ring buffer of syscall rate over time, rendered in the TUI footer and available via `/api/timeline`.
- **Integration tests with real strace** — add end-to-end tests that run `strace -T` on a dummy process and verify the full pipeline (parser → aggregator → report).
- **Parser benchmarks** — add `BenchmarkParse` to `internal/parser/` to measure throughput on realistic trace lines and catch regressions.

### Long-term (3+ months)

- **Themes / config file** — `~/.config/stracectl/config.toml` for colors, default flags, alert rules, and theme selection (dark/light/custom).
- **Periodic snapshots** — auto-checkpoint aggregator state to disk every N seconds. Useful for long-running traces where the TUI may crash or the session needs resumption.
- **Compact/granular TUI toggle** — `Tab` to switch between a summary view (top-N per category) and full syscall list.
- **Bookmark rows** — `b` to mark a TUI row for quick return. Useful in long traces.
- **Shell completions** — richer bash/zsh/fish completions for syscall names, file paths, and running PIDs.
- **External data streaming** — NDJSON sink to Kafka, Elasticsearch, or stdout for integration with observability pipelines. Extend the existing `--privacy-log` to support structured output modes.
- **OpenTelemetry exporter** — emit syscall events as OTel spans/metrics for correlation with application tracing systems.
- **Distributed tracing correlation** — if the traced process uses OTel/Jaeger, correlate strace events with trace IDs from the application.
- **Performance auto-analysis** — use aggregator data to generate recommendations (e.g., "syscall `openat` with 80ms avg latency → consider O_DIRECT or caching").
- **Plugin system** — let users define Lua or WASM plugins that process events in real time (custom alerting, filtering, enriching).
- **Stack traces (`--stack-trace`)** — capture per-syscall stack traces via BPF stack IDs + userspace symbolization (requires sampling to limit overhead).
- **Kernel-side decode-fds** — full BPF-based fd-to-path resolution (complex, platform-dependent).
- **Privacy linting** — run `semgrep` or similar in CI to verify the redaction pipeline catches all sensitive data patterns.
- **Richer errno docs** — `stracectl explain EACCES` should include a brief description and a pointer to `man errno`.
- **GoReleaser CI/CD** — GitHub Actions workflow with `goreleaser` for cross-platform binary releases.

---

## Implementation notes & code pointers

- CLI wiring: `cmd/run.go`, `cmd/attach.go`, `cmd/stats.go` — add flags and propagate options into tracer selection and tracer configuration.
- Strace subprocess: [internal/tracer/strace.go](internal/tracer/strace.go) — pass flags to the `strace` command and implement optional tee'ing of raw strace to disk.
- eBPF tracer: [internal/tracer/ebpf.go](internal/tracer/ebpf.go) — support reading filter maps, `Unfiltered`/`Force` already exist; extend to accept syscall and path filters, and prefer `EnterNs`/`ExitNs` for timestamps.
- Parser: [internal/parser/parser.go](internal/parser/parser.go) — add parsing of optional timestamp prefixes and support additional decoded fields if `strace` is invoked with `-y` or if eBPF provides `Path`.
- Models: [internal/models/event.go](internal/models/event.go) — extend `SyscallEvent` with optional fields like `DecodedFDs []string`, `Path string`, `StackTrace []string`, and `TimeSource` metadata.
- Persistence & reports: `internal/report/report.go`, `internal/server/server.go` — support NDJSON import/export and include `TopFiles`/timeline in HTML reports.
- Aggregation & UI: `internal/aggregator/*`, `internal/ui/*` — support per-pid aggregation, `TopFiles`, `TopSockets`, timeline ring buffer and TUI overlays.
- FD mapper: `internal/aggregator/fdmapper.go` — already implemented; needs CLI flag exposure and caching layer.
- Alerts: `internal/ui/render/alerts.go` — hardcoded matchers; extend to support user-defined threshold rules.

---

## Next steps (recommended order)

1. Implement `--save-events` (NDJSON) and add tests for both the `strace` tracer and `eBPF` tracer event serialization.
2. Add unified CLI flags (`--filter-syscalls`, `--trace-path`, `--string-limit`, `--status`) and wire them to both backends.
3. Implement `--timestamps=relative|absolute|ns` using `EnterNs` on eBPF and timestamp prefixes when parsing raw `strace` output.
4. Expose `--decode-fds` as an opt-in flag using the existing fdmapper with `/proc/<pid>/fd` resolution and caching.
5. Add server API endpoints and TUI overlays for `TopSockets` and the timeline sparkline.
6. Implement session save/load (`--save`) and diff (`stracectl diff`) for comparing traces across sessions.

---

## Estimated effort

| Tier | Scope | Estimate |
|---|---|---|
| Short-term | CLI flags, NDJSON export, timestamps | A few days of focused work |
| Mid-term | Session save/load, diff, configurable alerts, TopSockets, timeline, integration tests, parser benchmarks | 1–3 sprints |
| Long-term | Themes, plugins, OTel exporter, stack traces, kernel-side decodes, GoReleaser, privacy linting | Larger engineering projects requiring design and risk review |
