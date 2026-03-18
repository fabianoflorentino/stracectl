# Live tracing pipeline

This diagram shows the live tracing pipeline: events originate from either the eBPF tracer (kernel ringbuf) or the `strace` subprocess, are parsed, aggregated, and then consumed by the TUI, the HTTP sidecar, or exported as an HTML report.

```mermaid
flowchart TD
  subgraph Tracers
    EB["eBPF tracer\nringbuf (kernel events)"]
    ST["strace (subprocess)\nstderr — one line per syscall"]
  end

  PARSE["parser.Parse()"]
  AGG["aggregator.Add()\ndedicated goroutine · mutex-protected"]
  UI["ui.Run() — BubbleTea TUI (redraw every 200 ms)"]
  SRV["server.Start() — HTTP API, WebSocket, Prometheus"]
  REP["report.Write() — HTML report export"]

  ST -->|"chan SyscallEvent (buffered 4096)"| PARSE
  PARSE --> AGG
  EB -->|"chan SyscallEvent (ringbuf)"| AGG

  AGG -->|"default mode"| UI
  AGG -->|"--serve flag"| SRV
  AGG -->|"--report flag"| REP
```
