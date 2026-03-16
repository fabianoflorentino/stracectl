---
title: "Usage Scenarios"
description: "Practical troubleshooting examples with stracectl"
weight: 8
---

## Usage scenarios

This page presents practical examples showing how `stracectl` solves common
problems across different modes: TUI, sidecar/HTTP, attach, post-mortem
(`stats`), and eBPF.

### 1) Debug a local command (TUI)

Problem: a local utility (`curl`, `python`, `node`) is slow or failing.

Command:

```bash
sudo stracectl run --report report.html curl https://example.com
```

Open the TUI, look for `connect` or `read` rows, press `Enter` to view recent
error samples, and generate `report.html` to share findings.

### 2) Investigate a service in Kubernetes (sidecar + port-forward)

Use the sidecar (with `shareProcessNamespace: true`) and then `kubectl
port-forward` to access the UI locally:

```bash
kubectl apply -f deploy/k8s/sidecar-pod.yaml
kubectl -n <ns> port-forward pod/<sidecar-pod> 8080:8080
open http://localhost:8080
```

Tip: use `--container` or the `discover` subcommand to attach to the right
process inside the Pod. For safety, prefer `kubectl port-forward` over
exposing the endpoint publicly; if you must expose it, require `--ws-token`
and TLS.

### 3) Attach to a running PID

```bash
sudo stracectl attach 1234
```

`attach` is ideal for inspecting a process already running without restarting
it. To serve the UI instead, add `--serve :8080` and use `kubectl
port-forward`.

### 4) Post-mortem analysis (saved `strace`)

```bash
strace -T -o trace.log <command>
stracectl stats trace.log
stracectl stats --report report.html trace.log
```

Useful for incident response and for attaching observables to tickets without
recreating the environment.

### 5) Low-overhead tracing with eBPF

```bash
sudo stracectl run --backend ebpf --serve :8080 <command>
```

Notes: eBPF requires a compatible kernel (≥ 5.8), a binary built with
`-tags=ebpf`, and privileges to load BPF programs. Use eBPF when you need
lower observer impact.

### 6) Export metrics to Prometheus

Enable `--serve` and configure a `ServiceMonitor` via Helm or configure
Prometheus to scrape the sidecar:

```bash
helm upgrade stracectl ./deploy/helm/stracectl \
	--set serviceMonitor.enabled=true --set serviceMonitor.namespace=monitoring
```

Protect `/metrics` or restrict scraping to your monitoring network.

---

More examples and details are available in `docs/SCENARIOS.md` in the
repository.
