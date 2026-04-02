# Attach and container discovery

This diagram shows the attach workflow and container PID discovery: when `--container` is used, `discover.LowestPIDInContainer()` finds the target PID; `tracer.Select()` receives the `--backend` flag and selects the tracer (eBPF or strace), which then attaches to the PID and emits events into the aggregator.

```mermaid
flowchart TD
  USER["User: stracectl attach [--container <name>] [--backend auto|ebpf|strace] <pid>"]
  DISC["discover.LowestPIDInContainer()\n(container PID discovery)"]
  PID["Selected PID"]
  BACKEND["--backend flag\n(auto | ebpf | strace)"]
  SELECT["tracer.Select(backend)"]
  TRACER["EBPFTracer or StraceTracer\nAttach(ctx, pid)"]
  AGG["aggregator.Add()\ndedicated goroutine · mutex-protected"]
  UI["ui.Run() / server.Start() / report.Write()"]

  USER -->|"--container"| DISC --> PID
  USER -->|"<pid>"| PID
  BACKEND --> SELECT
  PID --> TRACER
  SELECT --> TRACER
  TRACER --> AGG
  AGG --> UI
```
