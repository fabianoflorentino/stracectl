# stracectl — Implementation Roadmap

This document describes the project roadmap, prioritized work items, and concrete implementation notes. It highlights features that should be supported consistently across both tracing backends (the classic `strace` subprocess and the eBPF tracer) and lists short-, mid-, and long-term priorities.

> **Last updated:** 2026-11-02
>
> **Tracking issue:** https://github.com/fabianoflorentino/stracectl/issues/61

---

## Product direction

The next stage for stracectl is to move from a syscall viewer to a troubleshooting copilot for Kubernetes and VM operators.

Three pillars guide the roadmap:

1. **Capture** — collect the right data with bounded overhead and safe defaults.
2. **Correlate** — connect syscalls with Kubernetes context, resource pressure, events, sessions and baselines.
3. **Explain** — turn raw events into prioritized findings and actionable next steps.

Target users:

- SREs and platform engineers debugging production workloads.
- Application developers investigating latency, errors and startup failures.
- VM operators troubleshooting systemd services, containers and host-level behavior.

Principles:

- Safe by default, with explicit opt-in for privileged access.
- Bounded memory/CPU use and clear drop/backpressure metrics.
- Consistent behavior across the `strace` and eBPF backends, with documented differences.
- Privacy-first capture, redaction and auditability.

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
- **Kubernetes sidecar and Helm chart** — sidecar manifest, Helm template and ServiceMonitor example.
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
- **Basic shell completion** — Cobra-generated completion for bash, zsh, fish and PowerShell.
- **Debian 13 distroless static runtime** — production image uses `gcr.io/distroless/static-debian13` with only the required glibc libraries. Removes `zlib` and OpenSSL from the runtime, reduces the image to about 9 MB and unblocks Docker Scout/releases (#49, #62).
- **Dependency and toolchain alignment** — Go 1.26.10 across `go.mod` and CI, `x/text` CVE fix, Dependabot updates and reproducible eBPF toolchain pinning (#43, #48).
- **eBPF test isolation** — stub-only tests are gated behind `!ebpf` so privileged integration runs are not mixed with non-eBPF assertions (#46).

---

## Tracking map

Every roadmap item should have a GitHub issue for tracking. The links below are the canonical references.

| Area | Item | Issue |
|---|---|---|
| Release | Debian 13 runtime and zlib removal | #49 (closed), #62 |
| Release | GoReleaser, SBOM and signing | #59 |
| CI | Go toolchain consistency | #50 |
| CI | Workflow and image validation | #51 |
| Security | Self-hosted runner hardening | #52 |
| Security | Sidecar RBAC, mTLS and audit | #70 |
| Testing | Benchmarks, fuzzing and coverage | #53 |
| Export | NDJSON and OpenTelemetry integration | #54 |
| CLI | Filters, capture bounds and timestamps | #55 |
| CLI | `--decode-fds` | #56 |
| Network/UI | TopSockets, DNS, network and timeline views | #57 |
| Sessions | Save/load/diff, baseline and alerts | #58 |
| Kubernetes | Context, events and resource-pressure correlation | #63 |
| UX | Guided triage and syscall/errno explanations | #64 |
| Reliability | Crash and startup-failure investigation | #65 |
| Kubernetes | kubectl plugin, ephemeral debug and Trace CRD | #66 |
| VM/Process | Process/container topology and systemd/cgroup attach | #67 |
| Platform | Central agent/collector mode | #68 |
| Platform | Plugin/extensibility system | #69 |
| Documentation | README, roadmap and site sync | #60 |
| Tracking | Engineering and product backlog | #61 |

---

## Cross-backend compatibility: strace-like options we will support

Goal: provide a unified CLI and consistent operator experience regardless of backend. When a feature cannot be implemented identically on both backends we will provide a best-effort behavior and document the difference.

- **Save parsed events (`--save-events` → NDJSON/JSON)** — write parsed events to disk in a structured format for offline analysis. See #54.
  - Supported by both backends: format events as NDJSON and include fields `time`, `pid`, `name`, `args`, `retval`, `error`, `latency`, plus optional `decoded_fds`, `path`, `stack` and context metadata when available.
  - Complexity: low.

- **Filter syscalls (`--filter-syscalls`)** — allow limiting traced syscalls or groups (file, net, process, memory) via a unified CLI flag. See #55.
  - strace: pass-through `-e trace=...`.
  - eBPF: implement a BPF map of enabled syscalls or apply a fast userspace filter after event emission.
  - Complexity: medium.

- **Path filtering (`--trace-path`)** — limit events to syscalls touching a path substring. See #55.
  - strace: pass-through `-P`.
  - eBPF: prefer matching on `Path` captured in the BPF event or filter in userspace; support writing path filters to a BPF map for kernel-side filtering later.
  - Complexity: medium.

- **String limit (`--string-limit`)** — control truncation of printed strings (read/write dumps). See #55.
  - strace: pass `-s N` to subprocess.
  - eBPF: truncate in userspace or recompile BPF with a larger buffer (requires BPF/C change).
  - Complexity: low.

- **Timestamps & precision (`--timestamps=relative|absolute|ns`)** — consistent timestamping across backends. See #55.
  - strace: parse timestamp prefixes when present.
  - eBPF: use `EnterNs`/`ExitNs` from BPF events and convert to wall-clock or relative time in userspace.
  - Complexity: medium.

- **Status filter (`--status=successful|failed`)** — show only errors or only successes. See #55.
  - Backend-agnostic: filter by `SyscallEvent.Error` in userspace.
  - Complexity: low.

- **Capture bounds (`--duration`, `--until`, `--max-events`, `--snapshot`)** — bound live investigations and produce point-in-time snapshots. See #55.
  - Applies to `run`, `attach`, `stats` and `--serve`.
  - Complexity: low.

- **FD decoding (`--decode-fds`)** — translate file descriptors into paths where possible. See #56.
  - strace: pass `-y` or implement userspace map of `/proc/<pid>/fd` lookups.
  - eBPF: prefer `Path` captured by BPF for common operations; fallback to resolving `/proc/<pid>/fd/<n>` in userspace.
  - The FD mapper (`internal/aggregator/fdmapper.go`) already exists; expose it as an opt-in CLI flag with caching.
  - Complexity: medium.

- **Explain syscall/errno (`stracectl explain syscall|errno`)** — describe common causes, impact and remediation hints. See #64.
  - Keep the existing privacy `explain` output available.
  - Surface explanations in the TUI detail view and HTML report.
  - Complexity: low.

Notes:

- Where kernel-side BPF filtering is not yet possible, prefer efficient userspace filtering to avoid resource blowup.
- For any backend difference the CLI will print a short diagnostic explaining the fallback behavior (e.g., "eBPF: path filtering applied in userspace").

---

## Roadmap priorities

### Short-term (next 1–2 weeks)

- **Kubernetes context enrichment** — auto-detect namespace, pod, container, node, workload, restart count, exit code and resource pressure; add labels to metrics, events, reports and NDJSON. See #63.
- **Guided triage** — `stracectl triage` runs a bounded capture and prints prioritized findings with next steps. See #64.
- **Syscall/errno explanations** — `stracectl explain syscall|errno` with causes and remediation hints. See #64.
- **Bounded investigations** — `--duration`, `--until`, `--max-events` and `--snapshot`. See #55.
- **Structured export** — `--save-events <file>` (NDJSON) for `run`, `attach` and `stats`, with a versioned schema. See #54.
- **Unified CLI flags** — `--filter-syscalls`, `--trace-path`, `--string-limit`, `--status` and `--timestamps`, wired to both backends. See #55.
- **`--decode-fds`** — expose the existing FD mapper as an opt-in flag with caching. See #56.

### Mid-term (1–3 months)

- **Network and DNS views** — TopSockets, DNS query/latency, connect/timeout/retry insights, service dependency mapping and network-focused filters. See #57.
- **Timeline and baseline** — syscall-rate and latency timeline plus baseline comparison against a healthy session. See #57 and #58.
- **Session save/load/diff** — `stracectl save`, `load` and `diff` for compressed aggregator state and before/after comparisons. See #58.
- **Configurable alerts and notifications** — rule syntax such as `--alert latency>50ms,errno=EACCES>10`, configuration-file support, webhook/Slack/PagerDuty delivery. See #58.
- **Crash and startup investigation** — `--capture-on-exit`, last-events context, restart watch and CrashLoopBackOff correlation. See #65.
- **Process/container topology** — process tree, cgroup/container grouping, per-thread breakdown and `--unit`/`--cgroup` selectors. See #67.
- **Kubernetes-native UX** — kubectl plugin, ephemeral debug container and Trace CRD/operator. See #66.
- **Sidecar security** — secure defaults, mTLS/reverse proxy, RBAC-aware authorization, audit and redaction preview. See #70.
- **Dynamic TUI filters** — filter by minimum latency, time range, fd or path substring. Extend the existing TUI filter (`/` key).
- **Integration tests with real strace** — end-to-end tests that run `strace -T` on a dummy process and verify the full pipeline.
- **Benchmarks and fuzzing** — parser/aggregator benchmarks plus fuzz targets for parser and event decoding. See #53.

### Long-term (3+ months)

- **Central agent/collector mode** — per-node agent streaming to a central collector with mTLS, RBAC, retention and store-and-forward. See #68.
- **Plugin/extensibility system** — Lua or WASM plugins for filters, alerts, enrichment and exporters with sandboxing. See #69.
- **OpenTelemetry and observability correlation** — OTel spans/metrics, Prometheus exemplars, Grafana dashboards and correlation with application tracing. See #54.
- **Themes / config file** — `~/.config/stracectl/config.toml` for colors, default flags, alert rules and theme selection.
- **Periodic snapshots** — auto-checkpoint aggregator state to disk every N seconds for long-running traces.
- **Compact/granular TUI toggle** — `Tab` to switch between a summary view and the full syscall list.
- **Bookmark rows** — `b` to mark a TUI row for quick return in long traces.
- **Richer shell completions** — dynamic completion for syscall names, file paths and running PIDs.
- **Distributed tracing correlation** — if the traced process uses OTel/Jaeger, correlate syscall events with application trace IDs.
- **Performance auto-analysis** — turn aggregator data into recommendations (e.g., "`openat` with 80 ms average latency; consider caching").
- **Stack traces (`--stack-trace`)** — capture per-syscall stack traces via BPF stack IDs and userspace symbolization, with sampling.
- **Kernel-side decode-fds** — full BPF-based fd-to-path resolution.
- **Privacy linting** — run Semgrep or similar in CI to verify redaction coverage.
- **GoReleaser, SBOM and signing** — cross-platform releases, checksums, SBOM, cosign signing and provenance. See #59.

---

## Implementation notes & code pointers

- CLI wiring: `cmd/run.go`, `cmd/attach.go`, `cmd/stats.go`, `cmd/triage.go` — add flags and propagate options into tracer selection and tracer configuration.
- Strace subprocess: [internal/tracer/strace.go](internal/tracer/strace.go) — pass flags to the `strace` command and implement optional tee'ing of raw strace to disk.
- eBPF tracer: [internal/tracer/ebpf.go](internal/tracer/ebpf.go) — support reading filter maps; `Unfiltered`/`Force` already exist; extend to accept syscall/path filters and prefer `EnterNs`/`ExitNs` for timestamps.
- Parser: [internal/parser/parser.go](internal/parser/parser.go) — add parsing of optional timestamp prefixes and support additional decoded fields if `strace` is invoked with `-y` or if eBPF provides `Path`.
- Models: [internal/models/event.go](internal/models/event.go) — extend `SyscallEvent` with optional fields such as `DecodedFDs []string`, `Path string`, `StackTrace []string`, `TimeSource` and Kubernetes context metadata.
- Persistence and reports: `internal/report/report.go`, `internal/server/server.go` — support NDJSON/OTel export, `TopFiles`/TopSockets/timeline and context labels in HTML reports.
- Aggregation and UI: `internal/aggregator/*`, `internal/ui/*` — support per-PID aggregation, ring buffers, baselines, TopFiles/TopSockets, timeline and TUI overlays.
- FD mapper: `internal/aggregator/fdmapper.go` — already implemented; needs CLI flag exposure and a caching layer.
- Kubernetes context: new `internal/k8s` package (or `internal/context`) plus server/CLI wiring; read metadata via the Downstream API or Kubernetes API with read-only RBAC.
- Crash capture: ring buffer in `internal/aggregator` with flush-on-exit wiring in `cmd/run.go` and `cmd/attach.go`; add `cmd/watch.go`.
- Process topology: `internal/procinfo` plus cgroup/systemd parsing; expose TUI overlay and `/api/processes`.
- Alerts: `internal/ui/render/alerts.go` — replace hardcoded matchers with rule evaluation and notification sinks.
- Agent/collector: new `cmd/agent.go`, `cmd/collector.go`, a versioned wire protocol and mTLS configuration.
- Plugins: new `internal/plugin` package with a documented API, sandboxing and example plugins.
- Security: `internal/server` — add mTLS, RBAC-aware handlers, audit logging and secure-default binding.

---

## Next steps (recommended order)

1. Implement Kubernetes context enrichment and expose it in metrics, events, reports and the API (#63).
2. Add `stracectl triage` and syscall/errno explanations (#64).
3. Add network/DNS, TopSockets and timeline views (#57).
4. Add bounded captures (`--duration`, `--until`, `--max-events`, `--snapshot`) (#55).
5. Implement `--save-events` (NDJSON) and the OpenTelemetry exporter (#54).
6. Add capture-on-exit and restart investigation (#65).
7. Implement session save/load/diff, baseline comparison and configurable alerts (#58).
8. Add process/container topology and `--unit`/`--cgroup` selectors (#67).
9. Build the kubectl plugin and Trace CRD/operator (#66).
10. Harden sidecar exposure with RBAC, mTLS and audit (#70).
11. Build the central agent/collector and plugin system (#68, #69).

---

## Estimated effort

| Tier | Scope | Estimate |
|---|---|---|
| Short-term | K8s context, triage/explain, bounded captures, NDJSON, unified flags | A few days to 1 sprint |
| Mid-term | Network/DNS/timeline, crash investigation, sessions/baseline/alerts, topology, K8s-native UX, sidecar security | 2–4 sprints |
| Long-term | Agent/collector, plugins, OTel/Grafana correlation, stack traces, kernel-side decodes, GoReleaser/SBOM | Larger projects requiring design and security review |
