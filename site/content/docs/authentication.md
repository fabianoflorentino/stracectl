---
title: "WebSocket Authentication"
description: "How to enable and use token authentication for the /stream WebSocket endpoint."
weight: 6
---

## WebSocket Token Authentication

To protect the `/stream` WebSocket endpoint from unauthorized access, you can require a shared authentication token.

### Quick start

- Start the server with `--ws-token <token>` (any command with `--serve`):

```bash
./stracectl --serve --ws-token "SUPER_SECRET_TOKEN"
```

- Or pass the token from an environment variable in the shell:

```bash
WS_TOKEN=SUPER_SECRET_TOKEN ./stracectl --serve --ws-token "$WS_TOKEN"
```

- If `--ws-token` is not set, the endpoint remains open (default behavior).

### Client examples

Prefer sending the token in the `Authorization: Bearer <token>` header when the client supports headers.

- `wscat` (header):

```bash
wscat -c ws://localhost:8080/stream -H "Authorization: Bearer SUPER_SECRET_TOKEN"
```

- `wscat` (query string):

```bash
wscat -c ws://localhost:8080/stream?token=SUPER_SECRET_TOKEN
```

- Node.js (`ws`):

```js
const WebSocket = require('ws');
const ws = new WebSocket('ws://localhost:8080/stream', {
  headers: { Authorization: 'Bearer SUPER_SECRET_TOKEN' }
});
ws.on('open', () => console.log('connected'));
```

- Browser note: browsers do not allow setting custom headers on WebSocket connections. Use the query string or a proxy that injects `Authorization`.

```js
// Browser example (less secure — query string):
const ws = new WebSocket('wss://example.com/stream?token=SUPER_SECRET_TOKEN');
```

### Kubernetes / containers (example)

Create a Secret and expose it to the Pod as an environment variable, then expand it in the container command:

```bash
kubectl create secret generic stracectl-ws-token --from-literal=ws-token=SUPER_SECRET_TOKEN
```

Example `Deployment` fragment (expand env var in `command`):

```yaml
env:
  - name: WS_TOKEN
    valueFrom:
      secretKeyRef:
        name: stracectl-ws-token
        key: ws-token
command: ["/bin/sh", "-c", "exec /usr/local/bin/stracectl --serve --ws-token \"$WS_TOKEN\""]
```

### Security considerations

- Prefer the `Authorization: Bearer` header when possible.
- Tokens in the query string may leak via logs, referer headers, or browser history — if used, always combine with TLS (`wss://`).
- The token is not generated automatically — store, rotate and rotate securely.
- The web dashboard does not prompt for the token; protect the dashboard using a reverse proxy or add UI support.

### Permissions & Capabilities

`stracectl` performs syscall tracing and may require additional privileges depending on the backend and runtime mode:

- **ptrace / strace tracer:** typically requires `CAP_SYS_PTRACE` or running as `root`. For local testing, running with `sudo` is the simplest option:

```bash
sudo stracectl run curl https://example.com
```

- **eBPF backend:** loading BPF objects usually requires elevated privileges or capabilities such as `CAP_BPF` and (on some kernels) `CAP_PERFMON` or `CAP_SYS_ADMIN`. When building/running eBPF locally ensure the binary is built with eBPF support and the runtime has the required capabilities.

- **Containers:** add the necessary capabilities to the container and disable seccomp if it blocks required syscalls. Example (ptrace + permissive seccomp):

```bash
docker run --rm --cap-add SYS_PTRACE --security-opt seccomp=unconfined \
  fabianoflorentino/stracectl:latest run curl https://example.com
```

- **systemd:** grant ambient capabilities in the unit file and allow them to be inherited by the process. Example drop-in snippet:

```
[Service]
ExecStart=/usr/local/bin/stracectl --serve :8080
NoNewPrivileges=false
AmbientCapabilities=CAP_SYS_PTRACE CAP_BPF
```

- **Kubernetes / sidecar:** the Helm chart and manifests already set the recommended `securityContext` (see the Kubernetes docs). Required settings include `shareProcessNamespace: true`, adding `SYS_PTRACE` to `capabilities.add`, and a seccomp profile that permits `ptrace`.

See: [Installation]({{< relref "docs/install.md" >}}) and [Kubernetes]({{< relref "docs/kubernetes.md" >}}) for full examples.

### Testing

Use `wscat`, `websocat` or a small Node.js script (above) to verify token-based authentication.

For further reading, see the [Usage Guide]({{< relref "usage.md" >}}) and the [ROADMAP](https://github.com/fabianoflorentino/stracectl/blob/main/docs/ROADMAP.md).

### Debugging

If you need verbose tracer diagnostics for parser troubleshooting, add the `--debug`
flag to the `stracectl` command. This enables noisy raw-strace diagnostics that may
help diagnose edge cases (for example, `EAGAIN` events with empty argument lists).
Only enable `--debug` when actively troubleshooting; do not leave it enabled in
production environments.

Example (debugging mode):

```yaml
command: ["/bin/sh", "-c", "exec /usr/local/bin/stracectl --serve --debug --ws-token \"$WS_TOKEN\""]
```
