---
title: "Kubernetes"
description: "Running stracectl as a sidecar in Kubernetes Pods."
weight: 10
---

## Overview

`stracectl` ships with:

- A minimal **Dockerfile** based on `debian:bookworm-slim`
- **Raw Kubernetes manifests** under `deploy/k8s/`
- A **Helm chart** under `deploy/helm/stracectl/`

The sidecar pattern works by sharing the **PID namespace** of the Pod. With
`shareProcessNamespace: true`, the `stracectl` container can see every process
inside the Pod — including those belonging to your app container — and use
`ptrace(2)` to intercept their syscalls. No shell access, no code changes, no
restarts required.

## How it works

{{< k8s-sidecar-diagram >}}

`ptrace` in attach mode is **non-intrusive**: it observes syscall entry/exit
without modifying the target process's execution.

## Prerequisites

- Kubernetes 1.19+ recommended.

Three settings are **required** on the sidecar container:

| Setting | Value | Why |
| --------- | ------- | ----- |
| `spec.shareProcessNamespace` | `true` | Without this each container has its own PID namespace; the sidecar cannot see the app's processes |
| `capabilities.add` | `[SYS_PTRACE]` | The Linux capability that allows `strace` to call `ptrace(2)` on another process |
| `seccompProfile.type` | `Unconfined` | The default Kubernetes seccomp profile blocks the `ptrace` syscall; it must be disabled on the sidecar |

> `runAsUser: 0` is also required because the `strace` binary needs root to
> attach to processes owned by other UIDs.

## Quick start with raw manifests

```bash
kubectl apply -f deploy/k8s/sidecar-pod.yaml
```

The manifest (`deploy/k8s/sidecar-pod.yaml`) creates a Pod with two containers:
the app placeholder and the hardened `stracectl` sidecar. Replace `myapp:latest`
with your real image.

## Step-by-step guide

**1. Apply the manifest or Helm chart** (see below).

**2. Attach and serve** — the manifest already passes `--serve :8080 --container app` so the
sidecar starts the HTTP API automatically and attaches to the container named `app`. To run it manually:

```bash
kubectl exec <pod-name> -c stracectl -- \
  stracectl attach --serve :8080 --container myapp
```

To enable verbose tracer diagnostics for troubleshooting, include `--debug` in the
command (either when exec'ing into the sidecar or in the container `args`):

```bash
kubectl exec <pod-name> -c stracectl -- \
  stracectl attach --debug --serve :8080 --container myapp
```

**3. Forward the port and explore:**

```bash
kubectl port-forward pod/<pod-name> 8080:8080
```

| What | Command |
| ------ | --------- |
| Live web dashboard | `open http://localhost:8080` |
| All syscalls (JSON) | `curl localhost:8080/api/stats \| jq .` |
| One syscall detail (P95/P99, errno) | `curl localhost:8080/api/syscall/read \| jq .` |
| Process metadata + global stats | `curl localhost:8080/api/status \| jq .` |
| Last 500 raw events (JSON) | `curl localhost:8080/api/log \| jq .` |
| WebSocket live stream | `wscat -c ws://localhost:8080/stream` |
| Prometheus metrics | `curl localhost:8080/metrics` |

## Annotated sidecar spec

```yaml
spec:
  # Required: all containers in the Pod share one PID namespace.
  shareProcessNamespace: true
```

### Local access and security

Prefer `kubectl port-forward` for temporary, local access to the sidecar rather than exposing the service via a `Service`/`Ingress`. If you do expose the API beyond localhost for long-term monitoring, enforce authentication (for example via `--ws-token`), terminate TLS at the ingress/proxy, and avoid passing tokens in query strings. Also limit Prometheus scrape to your monitoring network or require authentication for `/metrics`.

Example (port-forward recommended):

```bash
# forward the sidecar to local port and open the dashboard locally
kubectl -n <ns> port-forward pod/<pod-name> 8080:8080
open http://localhost:8080
```

  containers:
  - name: app
    image: myapp:latest          # replace with your workload

  - name: stracectl
    image: fabianoflorentino/stracectl:v1.0.38
    args:
      - attach
      - --debug                # optional: enable verbose tracer diagnostics
      - --serve
      - ":8080"
      - --container
      - app
    ports:
      - name: http
        containerPort: 8080
    securityContext:
      runAsUser: 0               # strace must run as root to attach to other-UID processes
      runAsNonRoot: false
      privileged: false          # privileged is NOT required — only SYS_PTRACE is needed
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      seccompProfile:
        type: Unconfined         # default seccomp blocks ptrace(2); unconfined only on the sidecar
      capabilities:
        drop: [ALL]              # drop everything first
        add:  [SYS_PTRACE]      # then add only what is needed
    resources:
      requests: { cpu: "50m",  memory: "32Mi" }
      limits:   { cpu: "200m", memory: "64Mi" }
```

## Helm chart

```bash
helm install stracectl ./deploy/helm/stracectl \
  --set target.image=your-app:latest \
  --set serve.port=8080
```

Key values (`values.yaml`):

| Value | Default | Description |
| ------- | --------- | ------------- |
| `target.image` | — | Image of the workload container |
| `serve.port` | `8080` | Port for the HTTP / WebSocket API |
| `serve.enabled` | `true` | Enable HTTP sidecar mode |
| `resources.limits.memory` | `128Mi` | Memory limit for the sidecar |
| `serviceMonitor.enabled` | `false` | Create a Prometheus ServiceMonitor |

## HTTP API endpoints

| Endpoint | Method | Description |
| ---------- | -------- | ------------- |
| `/` | GET | Live HTML dashboard |
| `/api/syscalls` | GET | JSON snapshot of all aggregated syscalls |
| `/api/syscall/{name}` | GET | JSON detail for one syscall (P95/P99, errno breakdown, recent error samples) |
| `/api/status` | GET | Process metadata + global stats |
| `/api/log` | GET | Most recent 500 raw syscall events |
| `/stream` | WS | WebSocket live feed (`SyscallEvent` JSON) |
| `/metrics` | GET | Prometheus metrics |

## Prometheus + Grafana

## Prometheus metrics

A number of Prometheus metrics are exposed when running in sidecar mode:

| Metric | Type | Description |
| ------ | ---- | ----------- |
| `stracectl_syscall_calls_total` | Counter | Total invocations per syscall/category |
| `stracectl_syscall_errors_total` | Counter | Failed invocations per syscall/category |
| `stracectl_syscall_duration_seconds_total` | Counter | Cumulative kernel time per syscall |
| `stracectl_syscall_duration_avg_seconds` | Gauge | Average kernel time per syscall |
| `stracectl_syscall_duration_max_seconds` | Gauge | Peak kernel time per syscall |
| `stracectl_syscalls_per_second` | Gauge | Recent call rate |

## Prometheus + Grafana

Enable the `ServiceMonitor`:

```bash
helm upgrade stracectl ./deploy/helm/stracectl \
  --set serviceMonitor.enabled=true \
  --set serviceMonitor.namespace=monitoring
```

This creates a `ServiceMonitor` CRD that Prometheus Operator will scrape
automatically. Import the provided Grafana dashboard JSON for a
pre-built syscall breakdown view.

## Security considerations

- `seccompProfile: Unconfined` applies **only** to the `stracectl` sidecar container, not to the app.
- `privileged: false` — the sidecar does **not** need full privileged mode, only `SYS_PTRACE`.
- **Pod Security Standards**: namespaces at the `restricted` level will block this pod.
  Use `baseline` or `privileged` PSS for the namespace (or bind the exception to the workload's
  `ServiceAccount`) when deploying observability tooling.
- `ptrace` in attach mode is non-intrusive: it observes syscall entry/exit without altering
  the target process's behaviour or memory.
