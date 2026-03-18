# Attach and container discovery

This diagram shows the attach workflow and container PID discovery: when `--container` is used, `discover.LowestPIDInContainer()` finds the target PID; the selected tracer (eBPF or strace) then attaches to the PID and emits events into the aggregator.

```mermaid
flowchart TD
  USER["User: stracectl attach [--container <name>] <pid>"]
  DISC["discover.LowestPIDInContainer()\n(container PID discovery)"]
  PID["Selected PID"]
  SELECT["tracer.Select()"]
  TRACER["EBPFTracer or StraceTracer\nAttach(ctx, pid)"]
  AGG["aggregator.Add()\ndedicated goroutine · mutex-protected"]
  UI["ui.Run() / server.Start() / report.Write()"]

  USER -->|"--container"| DISC --> PID
  USER -->|"<pid>"| PID
  PID --> TRACER
  SELECT --> TRACER
  TRACER --> AGG
  AGG --> UI
```
