# Sidecar / Server mode

This diagram documents the sidecar (HTTP) mode: the server exposes the web dashboard, the `/stream` WebSocket (optional token authentication), JSON API endpoints, and Prometheus metrics. Clients include browsers (dashboard), WebSocket clients, and Prometheus scrapers.

```mermaid
flowchart TD
  RUN["Run with --serve :PORT"]
  AGG["aggregator (live or replay)"]
  SRV["server.New(addr, agg, wsToken)"]
  ROUTES["routes: /, /api/stats, /stream (WS), /metrics"]
  AUTH["/stream token auth\n(Authorization: Bearer or ?token)"]
  CLIENTS["Clients: Browser dashboard, wscat/websocket clients, Prometheus"]

  RUN --> SRV
  SRV --> ROUTES
  AGG --> SRV
  ROUTES --> AUTH --> CLIENTS
```
