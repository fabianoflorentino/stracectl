---
title: "What's next"
description: "Short summary of upcoming work and link to canonical roadmap."
weight: 15
---

The canonical, detailed roadmap for `stracectl` is maintained in the
repository under `docs/ROADMAP.md`. This page provides a short summary
of the most visible next items.

Planned highlights
------------------

- eBPF backend improvements: expand eBPF support (build tooling and
  robustness) and make eBPF the default when kernels support it.
- Kubernetes/sidecar polish: additional Helm values, improved ServiceMonitor
  defaults, and example manifests for common deployment patterns.
- Observability: more Prometheus metrics and Grafana dashboard refinements.

New cross-backend features (roadmap highlights)
----------------------------------------------

- Structured event export (`--save-events`): export parsed events as NDJSON
  for offline analysis and ingestion into observability pipelines. The NDJSON
  format includes `time`, `pid`, `name`, `args`, `retval`, `error`, and
  `latency` plus optional fields when available (`path`, `decoded_fds`,
  `stack`). Supported by both the `strace` subprocess and the eBPF tracer.
- Unified CLI filters and controls: expose consistent flags such as
  `--filter-syscalls`, `--trace-path`, `--string-limit`, and `--status`
  so users get the same behavior regardless of backend. Where kernel-side
  filtering isn't available for eBPF, a best-effort userspace filter will
  be applied and a short diagnostic will be printed.
- Follow forks (`-f`): ensure child processes are included consistently across
  backends.
- Timestamp normalization (`--timestamps=relative|absolute|ns`):
  standardize event timestamps across backends (use `EnterNs` from eBPF
  events or parsed strace prefixes when present).
- FD decoding and path resolution (`--decode-fds`): resolve file-descriptor
  targets via `/proc/<pid>/fd` when the tracer does not provide a `Path`.
  This is opt-in and uses a small cache to limit overhead.
- TopFiles / TopSockets / Timeline: server endpoints and TUI overlays to
  surface most-accessed files, active sockets, and a flamegraph-style
  syscall timeline (keys: `f` for files, `s` for sockets, timeline in
  the footer).
- Summary/stats compatibility (`-c`): reuse the existing `aggregator`
  to present `strace -c`-style summaries (calls, time, errors) for both
  backends.

Longer-term items (design & risk review)
----------------------------------------

- Stack traces per syscall (`--stack-trace`): capture stack ids in BPF and
  symbolize them in userspace — a larger, optional capability.
- Kernel-side advanced decoding: move more decode work into BPF for
  efficiency (complex and platform-dependent).
- Tampering/injection features: very high risk and gated behind explicit
  unsafe flags and warnings; evaluate only if required.

For the full implementation plan and detailed items, see the canonical
roadmap in the repository:

Recently delivered
------------------

- Per-PID grouping (`--per-pid`) is now available in `run`, `attach`, and
  `stats`, allowing syscall rows to be split by process ID.

[Full Roadmap (docs/ROADMAP.md)](https://github.com/fabianoflorentino/stracectl/blob/main/docs/ROADMAP.md)
