# Live tracing pipeline

This diagram shows the live tracing pipeline: events originate from either the eBPF tracer (kernel ringbuf → Go channel) or the `strace` subprocess (stderr lines parsed inline), are aggregated, and then consumed by the TUI, the HTTP sidecar, or exported as an HTML report.

```mermaid
flowchart TD
  subgraph Tracers
    EB["eBPF tracer\nkernal ringbuf → chan SyscallEvent (buffered 4096)"]
    ST["strace (subprocess)\nstderr — one line per syscall"]
  end

  PARSE["parser.Parse()\n(runs inside strace tracer goroutine)"]
  AGG["aggregator.Add()\ndedicated goroutine · mutex-protected"]
  UI["ui.Run() — BubbleTea TUI (redraw every 200 ms)"]
  SRV["server.Start() — HTTP API, WebSocket, Prometheus"]
  REP["report.Write() — HTML report export"]

  ST --> PARSE
  PARSE -->|"chan SyscallEvent (buffered 4096)"| AGG
  EB -->|"chan SyscallEvent (buffered 4096)"| AGG

  AGG -->|"default mode"| UI
  AGG -->|"--serve flag"| SRV
  AGG -->|"--report flag"| REP
```
