---
title: "stracectl"
description: "A modern strace with real-time htop-style TUI and Kubernetes sidecar mode"
---

## Modern strace. Real-time TUI. Kubernetes-ready

Aggregate syscalls live — counts, latencies, errors, anomalies — in an interactive
dashboard or HTTP sidecar.

{{< hero >}}

{{< sidecar >}}

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

{{< features >}}

### Learn more

- [Installation]({{< relref "docs/install.md" >}})
- [Usage & Quickstart]({{< relref "docs/usage.md" >}})
- [Kubernetes sidecar]({{< relref "docs/kubernetes.md" >}})
- [HTTP API]({{< relref "docs/api.md" >}})
- [Security]({{< relref "docs/security.md" >}})
- [Cenários de uso]({{< relref "docs/scenarios.md" >}})
