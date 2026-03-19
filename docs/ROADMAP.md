# stracectl — Implementation Roadmap

This document describes the project roadmap, prioritized work items, and concrete implementation notes. It highlights features that should be supported consistently across both tracing backends (the classic `strace` subprocess and the eBPF tracer) and lists short-, mid-, and long-term priorities.

---

## Cross-backend compatibility: strace-like options we will support

Goal: provide a unified CLI and consistent operator experience regardless of backend. When a feature cannot be implemented identically on both backends we will provide a best-effort behavior and document the difference.

- **Follow forks (`-f`)** — follow child processes and include their syscalls.
  - strace: pass `-f` to the subprocess.
  - eBPF: attach global tracepoints and filter by PGID (or multi-entry map); support `--per-pid` mode to group by PID.
  - Complexity: low → medium.

- **Filter syscalls (`-e trace=` / groups)** — allow limiting traced syscalls or groups (file, net, process, memory).
  - strace: pass-through `-e trace=...`.
  - eBPF: implement either a BPF map of enabled syscalls or apply a fast userspace filter after event emission.
  - Complexity: medium.

- **Path filtering (`-P path` / `--trace-path`)** — limit events to syscalls touching a path substring.
  - strace: pass-through `-P`.
  - eBPF: prefer matching on `Path` captured in the BPF event or filter in userspace; support writing path filters to a BPF map for kernel-side filtering later.
  - Complexity: medium.

- **String limit (`-s`)** — control truncation of printed strings (read/write dumps).
  - strace: pass `-s N` to subprocess.
  - eBPF: truncate in userspace or recompile BPF with a larger buffer (requires BPF/C change).
  - Complexity: low.

- **Timestamps & precision (`-t`, `-r`, `-tt`)** — consistent timestamping across backends.
  - strace: parse timestamp prefixes when present.
  - eBPF: use `EnterNs`/`ExitNs` from BPF events and convert to wall-clock or relative time in userspace.
  - Complexity: medium.

- **FD decoding (`-y` / `--decode-fds`)** — translate file descriptors into paths where possible.
  - strace: pass `-y` or implement userspace map of `/proc/<pid>/fd` lookups.
  - eBPF: prefer `Path` captured by BPF for common operations; fallback to resolving `/proc/<pid>/fd/<n>` in userspace.
  - Complexity: medium.

- **Status filter (`--status=successful|failed`)** — show only errors or only successes.
  - Backend-agnostic: filter by `SyscallEvent.Error` in userspace.
  - Complexity: low.

- **Save parsed events (`--save-events` → NDJSON/JSON)** — write parsed events to disk in a structured format for offline analysis.
  - Supported by both backends: format events as NDJSON and include fields `time`, `pid`, `name`, `args`, `retval`, `error`, `latency`, plus optional `decoded_fds`, `path`, `stack` when available.
  - Complexity: low.

- **Summary (`-c`)** — provide `strace -c`-style summaries (calls/time/errors) via existing `aggregator` / `stats` logic.
  - Backend-agnostic: reuse `aggregator` to produce the same summary output.
  - Complexity: low.

Notes:

- Where kernel-side BPF filtering is not yet possible, prefer efficient userspace filtering to avoid resource blowup.
- For any backend difference the CLI will print a short diagnostic explaining the fallback behavior (e.g., "eBPF: path filtering applied in userspace").

---

## Roadmap priorities

### Short-term (next 1–2 weeks)

- Add `--save-events <file>` (NDJSON) to `run`, `attach`, and `stats` so both backends can dump parsed events for offline analysis.
- Expose a small set of unified CLI flags: `--filter-syscalls`, `--trace-path`, `--string-limit`, `--status` and wire them to `strace` (pass-through) and eBPF (userspace/BPF-map where practical).
- Ensure `-f` (follow forks) works consistently; add `--per-pid` to switch between aggregated and per-pid stats.
- Add a `--timestamps=relative|absolute|ns` option that controls how timestamps are computed and displayed for both backends.

### Mid-term (1–3 months)

- Implement FD decoding (`--decode-fds`) by resolving `/proc/<pid>/fd` when BPF does not supply a path; add opt-in caching to limit overhead.
- Implement an efficient BPF map-based syscall whitelist for eBPF so `--filter-syscalls` can be kernel-side when available.
- Add server endpoints and TUI controls to surface `TopFiles`, `TopSockets`, and the syscall timeline sparkline.

### Long-term (3+ months)

- Optional stack traces per syscall (`--stack-trace`) via BPF stack ids + userspace symbolization (big work and sampling is recommended).
- Full-featured decode-fds in kernel-space (complex and platform-dependent).
- Fine-grained tampering/injection support: extremely risky; only if explicitly requested and gated behind strong warnings and a separate unsafe command.

---

## Implementation notes & code pointers

- CLI wiring: `cmd/run.go`, `cmd/attach.go`, `cmd/stats.go` — add flags and propagate options into tracer selection and tracer configuration.
- Strace subprocess: [internal/tracer/strace.go](internal/tracer/strace.go) — pass flags to the `strace` command and implement optional tee'ing of raw strace to disk.
- eBPF tracer: [internal/tracer/ebpf.go](internal/tracer/ebpf.go) — support reading filter maps, `Unfiltered`/`Force` already exist; extend to accept syscall and path filters, and prefer `EnterNs`/`ExitNs` for timestamps.
- Parser: [internal/parser/parser.go](internal/parser/parser.go) — add parsing of optional timestamp prefixes and support additional decoded fields if `strace` is invoked with `-y` or if eBPF provides `Path`.
- Models: [internal/models/event.go](internal/models/event.go) — extend `SyscallEvent` with optional fields like `DecodedFDs []string`, `Path string`, `StackTrace []string`, and `TimeSource` metadata.
- Persistence & reports: `internal/report/report.go`, `internal/server/server.go` — support NDJSON import/export and include `TopFiles`/timeline in HTML reports.
- Aggregation & UI: `internal/aggregator/*`, `internal/ui/*` — support per-pid aggregation, `TopFiles`, `TopSockets`, timeline ring buffer and TUI overlays.

---

## Next steps (recommended order)

1. Implement `--save-events` (NDJSON) and add tests for both the `strace` tracer and `eBPF` tracer event serialization. This is low risk and immediately useful.
2. Add unified CLI flags (`--filter-syscalls`, `--trace-path`, `--string-limit`, `--status`) and wire them to the `strace` subprocess and to the eBPF tracer via userspace filtering.
3. Implement timestamp normalization using `EnterNs` on eBPF and timestamp prefixes when parsing raw `strace` output.
4. Add `--decode-fds` as an opt-in userspace resolver calling `/proc/<pid>/fd` and a basic cache.
5. Add server API endpoints for top files/sockets and the timeline, and a TUI overlay to expose them (keys: `f` for files, `s` for sockets, timeline in footer).

---

## Estimated effort

- Short-term items: small (a few days of focused work).
- Mid-term items: moderate (1–3 sprints), mostly integration and testing effort.
- Long-term items: larger engineering projects (symbolization, kernel-side decodes, safe tampering), require design and risk review.

---

If you want, I can implement the highest-impact short-term item now: add `--save-events` (NDJSON) and the CLI flags to wire `--string-limit` and `--status` through both backends. Say which flags or behavior you want prioritized and I will start a change set.
