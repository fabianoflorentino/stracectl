---
title: "WebSocket Authentication"
description: "How to enable and use token authentication for the /stream WebSocket endpoint."
weight: 5
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

### Testing

Use `wscat`, `websocat` or a small Node.js script (above) to verify token-based authentication.

For further reading, see the [Usage Guide]({{< relref "usage.md" >}}) and the [ROADMAP](https://github.com/fabianoflorentino/stracectl/blob/main/docs/ROADMAP.md).
