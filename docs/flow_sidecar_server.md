# Sidecar / Server mode

This diagram documents the sidecar (HTTP) mode: the server exposes the web dashboard, the `/stream` WebSocket (optional token authentication), JSON API endpoints, Prometheus metrics, and debug/pprof endpoints. Clients include browsers (dashboard), WebSocket clients, and Prometheus scrapers.

```mermaid
flowchart LR
  RUN["--serve :PORT"]
  AGG["aggregator\n(live or replay)"]
  SRV["server.New\n(addr, agg, wsToken)"]

  RUN --> SRV
  AGG --> SRV

  subgraph UI["Dashboard"]
    R_ROOT["/"]
    R_STATIC["/static/dashboard.js"]
    R_SYSCALL_PAGE["/syscall/{name}"]
  end

  subgraph API["JSON API"]
    R_API["/api — list endpoints"]
    R_STATUS["/api/status"]
    R_STATS["/api/stats"]
    R_LOG["/api/log"]
    R_CATEGORIES["/api/categories"]
    R_FILES["/api/files"]
    R_SYSCALL["/api/syscall/{name}"]
  end

  subgraph STREAM["WebSocket"]
    R_STREAM["/stream"]
    AUTH["token auth\n(Bearer or ?token=)"]
    R_STREAM --> AUTH
  end

  subgraph OBS["Observability"]
    R_METRICS["/metrics — Prometheus"]
    R_HEALTHZ["/healthz"]
    R_DEBUG["/debug/goroutines"]
    R_PPROF["/debug/pprof/*"]
  end

  SRV --> UI
  SRV --> API
  SRV --> STREAM
  SRV --> OBS

  UI --> BROWSER["Browser"]
  AUTH --> WSCLIENT["WebSocket clients"]
  R_METRICS --> PROM["Prometheus scraper"]
```
