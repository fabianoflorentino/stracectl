# stracectl — Functional and Non-Functional Requirements

## Functional Requirements

1. CLI Commands
   - Provide `run <cmd>` to start and trace a new command.
   - Provide `attach <pid>` to attach to a running process.
   - Provide `discover <container>` to locate the lowest PID in a container/Pod.
   - Provide `stats <file>` to replay and analyse `strace -T -o` logs.
   - Global flags: `--serve`, `--report <file>`, `--backend {ebpf,strace}`, `--ws-token`, `--debug`.

2. Tracing Backends
   - Support an eBPF backend when kernel and privileges allow.
   - Support a fallback `strace` subprocess tracer if eBPF is unavailable.
   - Allow forcing backend selection via `--backend` flag.

3. Parsing & Event Model
   - Convert raw tracer output lines into a structured `SyscallEvent` model with timestamp, syscall name, category, args, return value, errno, duration, pid/tid, and optional fd→path mapping.
   - Handle corrupted or partial trace lines robustly and surface parse errors when `--debug` is enabled.

4. Aggregation & Metrics
   - Aggregate syscall counts per syscall name and category.
   - Compute latency metrics: avg, max, total, and tail quantiles (p95, p99) per syscall.
   - Maintain per-errno counts and a recent error sample buffer per syscall.
   - Track top N files per syscall (where applicable) and aggregate per-file I/O counts.

5. Outputs
   - TUI: interactive BubbleTea dashboard showing aggregated metrics, categories, and a detail overlay for a selected syscall.
   - Sidecar HTTP Server: JSON endpoints for aggregated data, a `/stream` WebSocket endpoint for live deltas, and an embedded HTML dashboard.
   - Prometheus: expose metrics on `/metrics` with counters, histograms, and gauges.
   - Report: export a self-contained HTML report via `--report` at exit or on demand.

6. WebSocket Behavior
   - Support optional token authentication for `/stream` via `--ws-token`.
   - Enforce write deadlines and per-client send queue limits; drop slow clients and record metrics for client drops.

7. Replay Mode
   - `stats <tracefile>` should reuse the same parser→aggregator→output pipeline to produce identical views and reports from saved `strace -T -o` logs.

8. Discovery & Attach
   - `discover` must resolve the appropriate PID inside containers using `/proc/<pid>/cgroup` heuristics.
   - `attach` should validate permissions and produce clear error messages when attaching fails.

9. Debug & Diagnostics
   - Expose `/debug/pprof` and `/debug/goroutines` handlers when `--debug` is enabled (or bound to localhost for safety).


## Non-Functional Requirements

1. Performance
   - The tracing pipeline must impose minimal overhead on traced processes; aggregator hot-paths should avoid unnecessary allocations.
   - The system should handle workloads up to high syscall rates (design target depends on environment) with bounded memory and CPU; implement adaptive sampling if limits are exceeded.
   - Latency histograms must provide accurate tail estimations (p95/p99) with reasonable memory cost.

2. Scalability
   - The sidecar server is single-instance per target Pod (no internal horizontal sharding required). For cluster-wide tracing, recommend per-pod sidecars or a separate infrastructure design.

3. Reliability
   - Provide graceful shutdown: stop tracer, drain parser/aggregator channels, flush reports, and close WS clients.
   - Automatic fallback from eBPF to `strace` on failure to attach or load BPF programs.
   - Track and expose drop/sampling metrics when the pipeline is overloaded.

4. Security
   - Require or support a `--ws-token` for WebSocket clients; bind to localhost by default or document secure exposure via a reverse proxy.
   - Sanitize and escape any content rendered in TUI and HTML reports to prevent terminal corruption and XSS.
   - For eBPF usage, document the capabilities and privileged environment required; prefer isolated ephemeral VMs or dedicated CI runners.
   - Container images should follow hardening best practices: minimal base image, non-root execution where possible, `securityContext` values in Kubernetes manifests.

5. Observability
   - Export Prometheus metrics for core counters, histograms, and gauges (see `SYSTEM_DESIGN.md`).
   - Provide structured logs with configurable verbosity via `--debug`.
   - Include pprof/debug endpoints accessible only to trusted hosts or localhost.

6. Maintainability
   - Keep a clear separation of concerns: tracer, parser, aggregator, server, UI, and report.
   - Provide unit and integration tests covering parser edge-cases, aggregator correctness, and server handlers. eBPF tests may run only on labeled self-hosted runners.

7. Portability & Compatibility
   - Target Linux systems only; document kernel and dependency requirements for eBPF (kernel ≥ 5.8, `clang`, `bpf2go`).
   - Provide a fallback path (`strace`) for environments without eBPF support.

8. Resource Constraints
   - Sidecar resource usage should be bounded and configurable (memory/CPU limits, collector scrape intervals, channel buffer sizes).

9. Privacy
   - Avoid logging sensitive data by default. Provide documentation and flags to control any data retention or export features.


## Acceptance Criteria

- Core commands (`run`, `attach`, `stats`, `discover`) function as documented in `README.md`.
- Sidecar serves JSON, WebSocket, and Prometheus endpoints and enforces token auth when configured.
- Aggregator produces accurate aggregated metrics and tail latency estimates within acceptable error bounds for test workloads.
- eBPF backend loads on compatible kernels and falls back to `strace` on failure.
- The system exposes metrics for backlog, drops, WS clients, and basic health indicators.
