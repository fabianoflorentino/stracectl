---
title: "Kubernetes"
description: "Running stracectl as a sidecar in Kubernetes Pods."
weight: 3
---

## Overview

`stracectl` ships with:

- A minimal **Dockerfile** based on `debian:bookworm-slim`
- **Raw Kubernetes manifests** under `deploy/k8s/`
- A **Helm chart** under `deploy/helm/stracectl/`

The sidecar pattern shares the PID namespace of the target container so
`stracectl` can attach to any process in the Pod without a shell.

## Quick start with raw manifests

```bash
kubectl apply -f deploy/k8s/sidecar-pod.yaml
```

The manifest creates a Pod with two containers sharing `hostPID: false`
and `shareProcessNamespace: true`. The `stracectl` container is granted
`CAP_SYS_PTRACE` via a restricted `securityContext`.

## Helm chart

```bash
helm install stracectl ./deploy/helm/stracectl \
  --set target.image=your-app:latest \
  --set serve.port=8080
```

Key values (`values.yaml`):

| Value | Default | Description |
|-------|---------|-------------|
| `target.image` | — | Image of the workload container |
| `serve.port` | `8080` | Port for the HTTP / WebSocket API |
| `serve.enabled` | `true` | Enable HTTP sidecar mode |
| `resources.limits.memory` | `128Mi` | Memory limit for the sidecar |
| `serviceMonitor.enabled` | `false` | Create a Prometheus ServiceMonitor |

## PID discovery

In a shared-PID-namespace Pod, use `stracectl discover` to find the target
process without guessing:

```bash
# Inside the stracectl sidecar
stracectl discover myapp
# → 42

stracectl attach --serve :8080 "$(stracectl discover myapp)"
```

`discover` searches `/proc` for a container whose `comm` or command line matches
the given name, handling namespace boundaries automatically.

## HTTP API endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Live HTML dashboard |
| `/api/syscalls` | GET | JSON snapshot of all aggregated syscalls |
| `/api/syscall/{name}` | GET | JSON detail for one syscall (P95/P99, errno breakdown, recent error samples) |
| `/api/status` | GET | Process metadata + global stats |
| `/api/log` | GET | Most recent 500 raw syscall events |
| `/ws` | WS | WebSocket live feed (`SyscallEvent` JSON) |
| `/metrics` | GET | Prometheus metrics |

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
