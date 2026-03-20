# Prometheus integration for stracectl

This document shows example Prometheus configuration, recording rules and alerts for scraping stracectl metrics.

Where to scrape

- stracectl exposes Prometheus metrics at `/metrics` when running with the HTTP server (e.g. `--serve :4321`).

Example `prometheus.yml` (see `deploy/prometheus/prometheus.example.yml`):

- Add a scrape job targeting your stracectl instance (HOST:PORT). Example `static_configs` is included.

Recording rules

- `deploy/prometheus/recording_rules.yml` contains example recording rules:
  - `stracectl:syscall:rate5m` — per-syscall call rate over 5m
  - `stracectl:syscall:rate5m:category` — per-category call rate over 5m
  - `stracectl:syscall:p95` — P95 latency if the app exports a histogram (see note)

Alerting rules

- `deploy/prometheus/alerting_rules.yml` contains example alerts:
  - `HighSyscallRate` — total syscall rate above a threshold
  - `HighSyscallRateBySyscall` — a single syscall with unexpectedly high rate
  - `HighSyscallLatencyP95` — P95 latency above a threshold (requires histogram)

PromQL examples

- Top 10 syscalls by rate (5m):

```bash
topk(10, sum by (syscall) (rate(stracectl_syscall_calls_total[5m])))
```

- Rate per category (5m):

```bash
sum by (category) (rate(stracectl_syscall_calls_total[5m]))
```

- P95 latency for a syscall (if histogram is exported):

```bash
histogram_quantile(0.95, sum(rate(stracectl_syscall_latency_seconds_bucket[5m])) by (le, syscall))
```

Notes & tuning

- Adjust thresholds in `alerting_rules.yml` to match your environment.
- If stracectl does not export latency histograms, the latency-based rules will not work; consider enabling histogram export or compute percentiles externally.
- For Kubernetes environments use a `ServiceMonitor` (Prometheus Operator) or appropriate service discovery instead of `static_configs`.

ServiceMonitor example (Prometheus Operator):

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: stracectl
  labels:
    release: prometheus
spec:
  selector:
    matchLabels:
      app: stracectl
  endpoints:
  - port: metrics
    path: /metrics
    interval: 15s
```

If you want, I can:

- add these files into a `deploy/prometheus/` directory (already added),
- create a `deploy/prometheus/ServiceMonitor.yml` for k8s, or
- add `recording_rules` to your Prometheus operator manifests.
