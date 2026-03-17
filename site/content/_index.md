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

<div style="margin-top:1rem;padding:14px;border-radius:8px;background:linear-gradient(180deg,#0f1724,#0b1220);border:1px solid #1f2a36;">
	<h3 style="margin:0 0 6px 0;color:#9fe6ff">HTTP Sidecar & API</h3>
	<p style="margin:0 0 10px 0;color:#b9c6d3;max-width:60ch">Run `stracectl` in sidecar mode to expose a lightweight HTTP+WebSocket API and Prometheus metrics. The dashboard includes an interactive API explorer and live WebSocket stream.</p>
	<div style="display:flex;gap:8px;align-items:center">
		<a href="{{< relref "docs/api.md" >}}" class="btn btn-primary">API Reference</a>
		<a href="/api" class="btn" target="_blank" rel="noopener noreferrer">Open /api (live)</a>
		<pre style="margin-left:auto;background:#071019;color:#a8d0e6;padding:8px;border-radius:6px;font-family:monospace;">curl -s 'http://localhost:8080/api' | jq</pre>
	</div>
</div>

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

<div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(260px,1fr));gap:18px;margin-top:16px">
	<div style="background:#0d1117;border:1px solid #1f2a36;border-radius:12px;padding:16px;min-height:120px;display:flex;flex-direction:column;justify-content:space-between">
		<div style="display:flex;gap:12px;align-items:flex-start">
			<div style="width:44px;height:44px;border-radius:8px;background:#071426;display:flex;align-items:center;justify-content:center;font-size:20px">🌐</div>
			<div>
				<h4 style="margin:0;color:#9fe6ff">HTTP Sidecar API</h4>
				<p style="margin:6px 0 0 0;color:#b9c6d3;max-width:42ch">Expose JSON, WebSocket, and Prometheus metrics. Includes an interactive API explorer and a live `/api` listing for discovery.</p>
			</div>
		</div>
		<div style="margin-top:12px;display:flex;gap:8px;align-items:center">
			<a href="{{< relref "docs/api.md" >}}" class="btn btn-primary">API Reference</a>
			<a href="/api" class="btn" target="_blank" rel="noopener noreferrer">Open /api (live)</a>
		</div>
	</div>
</div>

### Learn more

- [Installation]({{< relref "docs/install.md" >}})
- [Usage & Quickstart]({{< relref "docs/usage.md" >}})
- [Kubernetes sidecar]({{< relref "docs/kubernetes.md" >}})
- [HTTP API]({{< relref "docs/api.md" >}})
- [Security]({{< relref "docs/security.md" >}})
- [Cenários de uso]({{< relref "docs/scenarios.md" >}})
