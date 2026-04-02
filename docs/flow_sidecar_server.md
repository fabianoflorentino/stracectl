# Sidecar / Server mode

This diagram documents the sidecar (HTTP) mode: the server exposes the web dashboard, the `/stream` WebSocket (optional token authentication), JSON API endpoints, Prometheus metrics, and debug/pprof endpoints. Clients include browsers (dashboard), WebSocket clients, and Prometheus scrapers.

```mermaid
flowchart TD
  RUN["Run with --serve :PORT"]
  AGG["aggregator (live or replay)"]
  SRV["server.New(addr, agg, wsToken)"]

  subgraph ROUTES["Routes"]
    R_ROOT["/ — web dashboard"]
    R_STATIC["/static/dashboard.js"]
    R_HEALTHZ["/healthz — health check"]
    R_API["/api, /api/ — list endpoints"]
    R_STATUS["/api/status — trace info"]
    R_STATS["/api/stats — syscall statistics"]
    R_LOG["/api/log — recent events"]
    R_CATEGORIES["/api/categories — category breakdown"]
    R_FILES["/api/files — top opened files"]
    R_SYSCALL["/api/syscall/{name} — single syscall stats"]
    R_SYSCALL_PAGE["/syscall/{name} — per-syscall SPA page"]
    R_STREAM["/stream — WebSocket live stats"]
    R_METRICS["/metrics — Prometheus metrics"]
    R_DEBUG["/debug/goroutines — goroutine/mem info"]
    R_PPROF["/debug/pprof/* — pprof profiling suite"]
  end

  AUTH["/stream token auth\n(Authorization: Bearer or ?token=)"]
  CLIENTS["Clients: Browser dashboard, wscat/websocket clients, Prometheus"]

  RUN --> SRV
  AGG --> SRV
  SRV --> ROUTES
  R_STREAM --> AUTH --> CLIENTS
  R_ROOT --> CLIENTS
  R_METRICS --> CLIENTS
```
