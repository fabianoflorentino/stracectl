---
title: "What's next"
description: "Short summary of upcoming work and link to canonical roadmap."
weight: 15
---

The canonical, detailed roadmap for `stracectl` is maintained in the
repository under `docs/ROADMAP.md`. The GitHub tracking issue for the current
backlog is
[#61](https://github.com/fabianoflorentino/stracectl/issues/61).

Product direction
-----------------

`stracectl` is moving from a syscall viewer to a troubleshooting copilot for
Kubernetes and VM operators. The roadmap is organized around three pillars:

- Capture: collect the right data with bounded overhead and safe defaults.
- Correlate: connect syscalls with Kubernetes context, events, sessions and baselines.
- Explain: turn raw events into prioritized findings and next steps.

Short-term highlights
---------------------

- Kubernetes context enrichment: namespace, pod, container, node, workload,
  restart count, exit code and resource-pressure correlation (#63).
- Guided triage: `stracectl triage` plus syscall/errno explanations with
  remediation hints (#64).
- Bounded investigations: `--duration`, `--until`, `--max-events` and
  `--snapshot` (#55).
- Structured export: NDJSON and OpenTelemetry integration (#54).
- Unified cross-backend filters and timestamp controls (#55).

Mid-term highlights
-------------------

- Network and DNS views: TopSockets, connect/timeout/retry insights, service
  mapping and timeline (#57).
- Crash and startup investigation with capture-on-exit and restart watch (#65).
- Session save/load/diff, baseline comparison and configurable alerts (#58).
- Process/container topology and systemd/cgroup attach (#67).
- Kubernetes-native UX: kubectl plugin, ephemeral debug and Trace CRD (#66).
- Sidecar hardening: RBAC, mTLS, audit and secure defaults (#70).

Long-term highlights
--------------------

- Central agent/collector mode for multi-node troubleshooting (#68).
- Plugin/extensibility system for filters, alerts, enrichment and exporters (#69).
- OpenTelemetry, Grafana and exemplar correlation (#54).
- Per-syscall stack traces, kernel-side decoding and performance auto-analysis.
- GoReleaser, SBOM and signed releases (#59).

Recently delivered
------------------

- Debian 13 distroless static runtime: no `zlib` or OpenSSL in the production
  image, smaller image and successful Docker Scout/release pipeline (#49, #62).
- Go 1.26.10 toolchain alignment and `x/text` CVE fix (#43, #48).
- eBPF stub test isolation behind the `!ebpf` build tag (#46).
- Structured event export: `--save-events` writes versioned NDJSON from `run`,
  `attach` and `stats`, with privacy filtering and redaction (#54).
- Basic shell completion for bash, zsh, fish and PowerShell.

For the full implementation plan, code pointers and estimated effort, see the
canonical roadmap:

[Full Roadmap (docs/ROADMAP.md)](https://github.com/fabianoflorentino/stracectl/blob/main/docs/ROADMAP.md)
