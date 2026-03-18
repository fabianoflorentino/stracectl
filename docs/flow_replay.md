# Post-mortem / Replay flow

This diagram shows the post-mortem analysis flow: a `strace -T -o` file is parsed and fed into the same aggregation pipeline used by live tracing, enabling the TUI, the HTTP sidecar, and HTML report export on offline data.

```mermaid
flowchart TD
  FILE["trace.log\n(strace -T -o trace.log)"]
  PARSER["parser.Parse()"]
  AGG["aggregator.Add()\ndedicated goroutine · mutex-protected"]
  UI["ui.Run() — TUI (offline)"]
  SRV["server.Start() — HTTP API (offline)"]
  REP["report.Write() — HTML report export"]

  FILE --> PARSER --> AGG
  AGG -->|"default mode"| UI
  AGG -->|"--serve flag"| SRV
  AGG -->|"--report flag"| REP
```
