# Kubernetes Sidecar

## Prerequisites

- Kubernetes 1.19+
- `strace` available in the sidecar image (included in the published image)
- `shareProcessNamespace: true` on the Pod spec

> **Security note:** `CAP_SYS_PTRACE` is a powerful capability. Only use this
> in debug/staging namespaces, or protect it with `PodSecurityAdmission`.

## Quick start with raw manifests

```bash
# 1. Replace myapp:latest with your real app image in the manifest
kubectl apply -f deploy/k8s/sidecar-pod.yaml

# 2. Forward the port
kubectl port-forward pod/myapp-stracectl 8080:8080

# 3. Query
curl http://localhost:8080/healthz
curl http://localhost:8080/api/stats | jq .
curl http://localhost:8080/metrics
# wscat -c ws://localhost:8080/stream
```

## Discover a container PID

When `shareProcessNamespace: true` is set, all container processes are visible from
the sidecar. Use `--container` to automatically resolve the right PID.

> **cgroupv2 / containerd / kind note:** some CRI implementations (including
> containerd used by kind) store hex container IDs in `/proc/<pid>/cgroup`
> instead of human-readable names. `stracectl` detects this automatically and
> falls back to matching the container name against the process name (`comm`)
> and full command-line (`cmdline`). `--container app` therefore works in both
> classic and cgroupv2 environments as long as the target process name or
> cmdline contains the given string.

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

When running in sidecar mode, `/metrics` exposes core telemetry useful for
alerts and dashboards. Example metrics include:

| Metric | Type | Description |
| ------ | ---- | ----------- |
| `stracectl_syscall_calls_total{syscall,category}` | Counter | Total syscall invocations (labelled by `syscall` and `category`) |
| `stracectl_syscall_errors_total{syscall,errno}` | Counter | Failed invocations grouped by `syscall` and `errno` |
| `stracectl_syscall_latency_seconds_bucket` | Histogram (buckets) | Latency distribution for syscall kernel time (use with `histogram_quantile`) |
| `stracectl_ws_clients` | Gauge | Number of active WebSocket clients |
| `stracectl_tracer_backlog` | Gauge | Current tracer/parser backlog (channel depth) |

A `ServiceMonitor` for Prometheus Operator is included in
[`deploy/k8s/servicemonitor.yaml`](../deploy/k8s/servicemonitor.yaml) and can be
enabled via the Helm chart with `--set serviceMonitor.enabled=true`.

## Troubleshooting

### `exec /usr/local/bin/stracectl: no such file or directory`

The container starts but immediately exits with this error. This means the
binary was linked against glibc but the runtime image does not provide it.

**Cause:** the Docker image was built with the `distroless/static` base, which
contains no C runtime library. The `stracectl` binary is built with
`CGO_ENABLED=1` and requires glibc at runtime.

**Fix:** the published image uses `gcr.io/distroless/cc:nonroot` as its base,
which bundles glibc. If you are building your own image, make sure the
production stage is:

```dockerfile
FROM gcr.io/distroless/cc:nonroot AS production
```

---

### `no process found for container "X"`

`stracectl` starts but cannot locate the target container process and exits
(or never starts tracing).

**Causes and fixes:**

1. **Container not yet ready** — the sidecar may start before the app container
  process is visible at `/proc`. The manifest already uses a
  `livenessProbe`/`readinessProbe`; if you run `stracectl` manually, add a
  short retry loop or `sleep` before the `attach` call.

2. **Name does not match comm or cmdline** — `--container X` is matched against
  the process name (`/proc/<pid>/comm`, up to 15 characters) and the command
  line (`/proc/<pid>/cmdline`). Use the exact executable base name:

  ```bash
  # Inspect what names are visible in the sidecar:
  kubectl exec <pod> -c stracectl -- stracectl discover X
  ```

  If the app runs as `python3 app.py`, use `--container python3`, not
  `--container app`.

3. **cgroupv2 / containerd / kind** — if cgroup paths carry hex container IDs
  (e.g. `cri-containerd-aabbccdd.scope`), the cgroup scan finds nothing.
  `stracectl` falls back automatically to comm/cmdline matching; if that also
  fails, confirm the container name as above.

---

### `ImagePullBackOff` on the sidecar container

Kubernetes cannot pull `fabianoflorentino/stracectl:<tag>`.

**Fixes:**

- Use `fabianoflorentino/stracectl:latest` or a known pinned tag (e.g.
  `v1.0.124`). Check available tags on
  [Docker Hub](https://hub.docker.com/r/fabianoflorentino/stracectl/tags).
- If running in a local kind cluster, load the image directly instead of
  pulling from the registry:

  ```bash
  kind load docker-image fabianoflorentino/stracectl:latest --name <cluster>
  ```

  Then set `imagePullPolicy: Never` on the sidecar container.
