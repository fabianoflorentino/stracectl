---
title: "stracectl"
description: "A modern strace with real-time htop-style TUI and Kubernetes sidecar mode"
---

## Modern strace. Real-time TUI. Kubernetes-ready.

Aggregate syscalls live — counts, latencies, errors, anomalies — in an interactive
dashboard or HTTP sidecar.

<picture>
	<source srcset="/img/hero.gif" type="image/gif">
	<img src="/img/hero.svg" alt="stracectl TUI preview" style="max-width:100%;height:auto;border-radius:6px;" />
</picture>

<p style="margin-top:0.75rem;">
	<a href="{{< relref "docs/install.md" >}}" class="btn btn-primary" style="margin-right:0.6rem;">Install</a>
	<a href="{{< relref "docs/quickstart.md" >}}" class="btn" style="margin-right:0.6rem;">Quickstart</a>
	<a href="{{< relref "docs/_index.md" >}}" class="btn">Read the docs</a>
</p>

### Quickstart

Run the included binary for a quick look at the TUI:

```bash
# local test (repo includes `bin/stracectl`)
./bin/stracectl --help

# or install via Go
go install github.com/fabianoflorentino/stracectl@latest
stracectl --help
```

### Key features

- Real-time aggregation: counts, latencies, error rates
- Interactive TUI: htop-style syscall dashboard
- HTTP sidecar & API: metrics and event export
- Multiple backends: `strace` parser, `ptrace` option, eBPF (low overhead)
- Kubernetes-ready: Helm chart and ServiceMonitor

### Learn more

- [Installation]({{< relref "docs/install.md" >}})
- [Usage & Quickstart]({{< relref "docs/usage.md" >}})
- [Kubernetes sidecar]({{< relref "docs/kubernetes.md" >}})
- [HTTP API]({{< relref "docs/api.md" >}})
- [Security]({{< relref "docs/security.md" >}})
