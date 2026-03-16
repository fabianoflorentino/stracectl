# Usage scenarios

This page collects practical examples that demonstrate how to use `stracectl`
to solve real-world problems. Each scenario describes the problem, the
commands to run, and why the chosen mode is appropriate.

1) Debug a local command (TUI)

- Problem: a CLI command (`curl`, `python`, `node`) is showing high latency or
  intermittent failures.
- Command:

```bash
sudo stracectl run --report report.html curl https://example.com
```

- What to do: open the TUI, inspect the `AVG` / `MAX` columns, filter for
  `connect` or `read`, and press `Enter` on a row to view recent error samples.
  Generate `report.html` for sharing after investigation.
- Why use: immediate interactive feedback with no artifact required; ideal for
  quick troubleshooting on a local machine.

2) Investigate a service in Kubernetes (sidecar + port-forward)

- Problem: a Pod in the cluster shows latency or errors and you want to inspect
  syscalls without disrupting the workload.
- Steps:

```bash
# apply the sidecar manifest or enable via Helm (e.g. deploy/k8s/sidecar-pod.yaml)
kubectl apply -f deploy/k8s/sidecar-pod.yaml

# forward the sidecar port to localhost
kubectl -n <ns> port-forward pod/<sidecar-pod> 8080:8080

# open the dashboard locally
open http://localhost:8080
```

- Tip: use `--container` or the `discover` subcommand to attach to the correct
  process inside the Pod. Security: prefer `kubectl port-forward` over exposing
  the endpoint publicly; if you must expose it, require `--ws-token` and TLS.

3) Attach to an already-running process (`attach`)

- Problem: inspect a specific PID on a host to investigate production behavior
  or reproduce a bug.
- Command:

```bash
sudo stracectl attach 1234
```

- What it does: attaches the TUI to the target process and starts live
  aggregation. To use the web UI instead, add `--serve :8080` and access via
  `kubectl port-forward`.

4) Post-mortem analysis with a saved `strace` (`stats`)

- Problem: you have a `strace -T -o trace.log` capture and want readable
  statistics or a report.
- Commands:

```bash
stracectl stats trace.log
stracectl stats --report report.html trace.log
# or serve results over HTTP
stracectl stats --serve :8080 trace.log
```

- Why: inspect captures without recreating the environment; useful for incident
  response and attaching observables to a ticket.

5) Low-overhead tracing with eBPF (when supported)

- Problem: monitor with lower overhead in production or under performance-
  sensitive loads.
- Example:

```bash
sudo stracectl run --backend ebpf --serve :8080 <command>
```

- Notes: eBPF requires a compatible kernel (≥ 5.8), a binary built with
  `-tags=ebpf`, and privileges to load BPF programs. Use eBPF when you need
  minimal observer impact.

6) Export metrics to Prometheus (continuous monitoring)

- Problem: aggregate syscall metrics in Prometheus/Grafana.
- Approach: enable `--serve` and use the `ServiceMonitor` via Helm or configure
  Prometheus to scrape the sidecar. Example:

```bash
helm upgrade stracectl ./deploy/helm/stracectl \\
  --set serviceMonitor.enabled=true --set serviceMonitor.namespace=monitoring
```

- Security: restrict scraping to the monitoring network or protect `/metrics`.

How to choose the mode

- `run` (TUI): quick interactive troubleshooting on a local host.
- `attach`: inspect a specific running PID.
- `stats`: offline analysis of logs and report generation.
- `--serve` (sidecar): remote debugging via the web dashboard; pair with
  `kubectl port-forward` for temporary access.
- `--backend ebpf`: use when you need low overhead and your environment supports it.

Useful links

- `docs/LOCAL_USAGE.md` — recommendations for binding, port-forwarding, and local security
- `docs/KUBERNETES.md` — manifest/Helm examples and securityContext guidance
