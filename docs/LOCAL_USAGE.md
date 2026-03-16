# Local security and usage

This page describes simple practices for running the `--serve` HTTP sidecar
locally without exposing the server and reducing the risk of accidental public
exposure.

Quick summary

- When possible, bind the server to `127.0.0.1` / `localhost`.
- To inspect a sidecar running in Kubernetes, prefer `kubectl port-forward`.
- If you must expose the server outside the host, require a token and TLS.
- Avoid passing tokens in query strings on public networks — prefer `Authorization`.
- Protect `/metrics`: allow scraping only from internal networks or require auth.

Why this matters

The `--serve` mode exposes JSON endpoints, a WebSocket stream, and Prometheus
metrics. If these endpoints are reachable publicly without authentication,
third parties could consume telemetry or open live WebSocket connections.
For quick, short-lived troubleshooting sessions, running the server locally
substantially reduces the attack surface.

Practical recommendations

1. Running locally (most secure and simplest)

```bash
# bind to localhost only
sudo stracectl run --serve 127.0.0.1:8080 <command>

# open the dashboard on the host: http://localhost:8080
```

2. Accessing a sidecar in a cluster — port-forward (recommended)

```bash
# forward the pod port to localhost
kubectl -n <ns> port-forward pod/<sidecar-pod> 8080:8080

# then open http://localhost:8080 as above
```

3. When exposing the server (long-term monitoring)

- Require `--ws-token` (or another authentication layer) and prefer the header
  `Authorization: Bearer <token>` — avoid tokens in query strings.
- Terminate TLS at the ingress or proxy and restrict origins (CORS / CheckOrigin).
- Protect `/metrics` by limiting Prometheus scrape to internal networks or by
  requiring authentication.

4. Notes about sidecars and capabilities

- To attach to processes in another container (sidecar), use
  `shareProcessNamespace: true` in the Pod spec and use `--container` or
  the `discover` subcommand to find the target PID. Grant only the minimal
  capabilities required.

Quick checklist before exposing an endpoint to untrusted networks

- [ ] Binding to `0.0.0.0`? → require `--ws-token` and TLS
- [ ] Is `/metrics` exposed? → restrict scrape or protect with auth
- [ ] Tokens passed in query strings? → avoid and document alternatives
- [ ] Public site contains dev scripts (livereload)? → remove and rebuild

Related links

- Usage guide: ../docs/USAGE.md
- Kubernetes / Helm: ../docs/KUBERNETES.md
