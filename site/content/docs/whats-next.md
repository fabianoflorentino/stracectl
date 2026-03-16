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

- Direct `ptrace` backend: replace the `strace` subprocess with a native
  `ptrace` implementation to remove the runtime dependency on the `strace`
  binary and enable richer structured events.
- eBPF backend improvements: expand eBPF support (build tooling and
  robustness) and make eBPF the default when kernels support it.
- Kubernetes/sidecar polish: additional Helm values, improved ServiceMonitor
  defaults, and example manifests for common deployment patterns.
- Observability: more Prometheus metrics and Grafana dashboard refinements.

For the full implementation plan and detailed items, see the canonical
roadmap in the repository:

[Full Roadmap (docs/ROADMAP.md)](../../docs/ROADMAP.md)
