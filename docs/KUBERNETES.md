# Kubernetes Sidecar

## Prerequisites

- Kubernetes 1.19+
- `strace` available in the sidecar image (included in the published image)
- `shareProcessNamespace: true` on the Pod spec

> **Security note:** `CAP_SYS_PTRACE` is a powerful capability. Only use this
> in debug/staging namespaces, or protect it with `PodSecurityAdmission`.

## Quick start with raw manifests

```bash
# 1. Edit the target container name in the manifest
kubectl apply -f deploy/k8s/sidecar-pod.yaml

# 2. Forward the port
kubectl port-forward pod/myapp-stracectl 8080

# 3. Query
curl http://localhost:8080/api/stats | jq .
curl http://localhost:8080/metrics
# wscat -c ws://localhost:8080/stream
```

## Discover a container PID

When `shareProcessNamespace: true` is set, all container processes are visible from
the sidecar. Use `--container` to automatically resolve the right PID:

```bash
stracectl attach --serve :8080 --container myapp
```

Or use `discover` to script around the PID:

```bash
stracectl discover myapp
# prints the lowest PID whose cgroup path matches "myapp"
```

## Helm chart

The Helm chart provides a `stracectl.sidecar` template you can include in your
existing Deployment:

```bash
# Install the chart (creates a ServiceMonitor if serviceMonitor.enabled=true)
helm install stracectl ./deploy/helm/stracectl \
  --set targetContainer=myapp \
  --set serviceMonitor.enabled=true
```

Add the sidecar to your Deployment template:

```yaml
spec:
  shareProcessNamespace: true
  template:
    spec:
      containers:
        - name: myapp
          image: myapp:latest
        {{- include "stracectl.sidecar" . | nindent 8 }}
```

## Prometheus metrics

When running in sidecar mode, `/metrics` exposes:

| Metric | Type | Description |
| ------ | ---- | ----------- |
| `stracectl_syscall_calls_total` | Counter | Total invocations per syscall/category |
| `stracectl_syscall_errors_total` | Counter | Failed invocations per syscall/category |
| `stracectl_syscall_duration_seconds_total` | Counter | Cumulative kernel time per syscall |
| `stracectl_syscall_duration_avg_seconds` | Gauge | Average kernel time per syscall |
| `stracectl_syscall_duration_max_seconds` | Gauge | Peak kernel time per syscall |
| `stracectl_syscalls_per_second` | Gauge | Recent call rate |

A `ServiceMonitor` for Prometheus Operator is included in
[`deploy/k8s/servicemonitor.yaml`](../deploy/k8s/servicemonitor.yaml) and can be
enabled via the Helm chart with `--set serviceMonitor.enabled=true`.
