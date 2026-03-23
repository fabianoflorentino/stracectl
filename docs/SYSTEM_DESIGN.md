# stracectl — System Design

## Context

- Project: `stracectl` — a modern strace replacement with a real-time, htop-style TUI and an HTTP "sidecar" mode exposing JSON, WebSocket, and Prometheus metrics.
- Primary runtime: Linux. Optional eBPF backend (Linux ≥ 5.8) or classic `strace` subprocess tracer.

## High-level Architecture

- Tracer: eBPF or `strace` subprocess produces raw syscall trace lines.
- Parser: converts tracer output lines into structured `SyscallEvent` objects.
- Aggregator: in-memory, thread-safe component that aggregates counts, latencies (avg, max, p95, p99), per-errno breakdowns, and top-file attributions.
- Outputs:
  - TUI (BubbleTea) interactive dashboard for local usage.
  - Server (sidecar) exposing JSON endpoints, WebSocket `/stream`, and Prometheus metrics plus an HTML dashboard.
  - Report renderer that generates a self-contained HTML report on demand or at exit.
- CLI (Cobra commands under `cmd/`) orchestrates tracing, attach/discover, replay, serve, and report modes.

## Data Flow

1. Tracer emits raw lines (from eBPF ring buffer or `strace` stdout).
2. Parser consumes lines, producing `SyscallEvent` values (timestamp, name, args, return value, errno, duration, pid/tid, optional fd→path info).
3. Aggregator receives `SyscallEvent` through channels and updates aggregated state.
4. Aggregator notifies downstream consumers:
   - UI render loop (pushes view updates at a fixed refresh rate).
   - WS broadcaster (pushes JSON deltas to connected clients, respecting write deadlines).
   - Prometheus collectors (exposes counters/histograms/gauges on `/metrics`).
   - Report writer (on-demand or at exit flush).
5. Optionally, replay mode feeds the same parser→aggregator pipeline from saved `strace -T -o` files.

## Components & Responsibilities

- `internal/tracer`: backend detection/selection, lifecycle, capability checks, and backpressure to parser.
- `internal/parser`: robust parsing, error handling for corrupted lines, conservative allocations for performance.
- `internal/aggregator`: concurrency-safe fast-paths (atomics where possible), histograms/quantile estimators for tail latencies, top-N data structures for files and syscalls.
- `internal/server`: HTTP handlers (JSON), WebSocket `/stream` with optional token auth, Prometheus instrumentation, and debug endpoints (`/debug/pprof`, `/debug/goroutines`).
- `internal/ui`: BubbleTea TUI, detail overlay, keybindings and sanitization to avoid terminal corruption.
- `internal/report`: HTML template rendering and embedding static assets.
- `cmd/`: CLI layer that wires tracer, parser, aggregator and outputs according to flags (`--serve`, `--report`, `--backend`, `--ws-token`, `--debug`).
- `deploy/`, `helm/`: Kubernetes manifests and Helm chart for sidecar deployment with hardened securityContext and ServiceMonitor.

## Concurrency and Backpressure

- Use bounded channels between stages (tracer → parser → aggregator). Track and expose backlog metrics.
- Apply drop or sampling policy when overwhelmed; surface drop metrics to Prometheus.
- Use atomic counters for hot metrics; protect complex structures (maps, top-N) with fine-grained mutexes.
- Per-WS-client send queues with write deadlines and per-client limits; drop slow clients and increment a metric.

## Data Model (key structs)

- `SyscallEvent`: timestamp, syscall name, category, duration, return value, errno, pid/tid, fd (optional), resolved path (optional), raw args.
- Aggregated record: key = (syscall name, category) → {calls, errors, avg, max, p95, p99, total time, per-errno counts, top-files}.

## Observability & Metrics

- Counters: `stracectl_syscall_calls_total{syscall,category}`, `stracectl_syscall_errors_total{syscall,errno}`.
- Histogram/Summary: `stracectl_syscall_latency_seconds` (buckets tuned microseconds→milliseconds).
- Gauges: `stracectl_ws_clients`, `stracectl_tracer_backlog`, `stracectl_parser_backlog`.
- Debug endpoints: `/debug/pprof`, `/debug/goroutines` (protected or localhost-only in production).
- Alerts: high error rate per syscall, sudden spike in p99 latency, sustained tracer backlog, high WS client churn.

## Security

- WebSocket token authentication via `--ws-token`. Bind the sidecar to localhost by default or require a reverse-proxy with mTLS for cluster exposure.
- Sanitize any user-supplied content shown in the TUI or HTML report to avoid terminal or XSS issues.
- For eBPF: require isolated privileged environment or minimal capabilities (documented), prefer running tests/CI in ephemeral VMs.
- Container hardening: run non-root where feasible, set restrictive `securityContext`, drop unnecessary capabilities, use read-only filesystem for static assets.

## Deployment Patterns

- Sidecar mode: deploy as a per-pod sidecar for live troubleshooting; use the Helm chart under `deploy/helm/stracectl/`.
- Standalone/debug: run as a one-off container/CLI tool on a privileged node for cluster-level traces (requires more careful hardening).
- CI eBPF jobs: use dedicated self-hosted runners with required kernel headers and capabilities (documented in README).

## Reliability & Operational Considerations

- Graceful shutdown: drain tracer, flush and persist report, close WS clients, and optionally write a final metrics snapshot.
- Fallbacks: prefer eBPF when available; on load failure or insufficient privileges, fall back to `strace` mode automatically.
- Sampling & adaptive strategies: implement adaptive sampling or rate-limiting for extremely high syscall rates to bound memory/CPU usage.

## Recommendations & Future Improvements

- Use a streaming quantile estimator (e.g., t-digest) or Prometheus histograms tuned for syscall latencies to reduce memory footprint while preserving tail accuracy.
- Add optional persistence (append-only log or lightweight time-series backend) for long-term analysis and correlation with external telemetry.
- Improve RBAC/audit for sidecar exposure in multi-tenant clusters; support integration with an API proxy for authentication and authorization.
- Provide a small export format (NDJSON) for exporting aggregated deltas to external systems for offline analysis.

## Next Steps

- Generate a visual architecture diagram (SVG) derived from this document.
- Optionally: raise a PR with recommended telemetry metrics and a small backpressure metric if you want me to implement it.
