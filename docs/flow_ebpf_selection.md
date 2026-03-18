# Backend selection: eBPF vs Strace

This diagram describes how the tracing backend is selected. The `--backend` flag can force a backend; otherwise `tracer.Select()` uses runtime checks (build tag and kernel version) to choose eBPF when available, and falls back to the `strace` subprocess tracer. The `--force-ebpf` option causes a hard failure on eBPF probe errors.

```mermaid
flowchart TD
  USER["User: --backend flag\n(auto | ebpf | strace)"]
  SELECT["tracer.Select()"]
  EBPF_BUILD["built with 'ebpf' build tag?"]
  KERNEL["kernel >= 5.8? (uname)"]
  EBPF_AVAIL["ebpfAvailable()\n(build tag && kernel check)"]
  EB["EBPFTracer (Run/Attach)"]
  ST["StraceTracer (subprocess)"]
  FORCE["--force-ebpf: fail-fast on eBPF probe error"]
  FALLBACK["fallback to Strace when probe fails (unless forced)"]

  USER --> SELECT
  SELECT -->|"auto"| EBPF_AVAIL
  SELECT -->|"ebpf"| EB
  SELECT -->|"strace"| ST

  EBPF_AVAIL -->|"true"| EB
  EBPF_AVAIL -->|"false"| ST

  EB -->|"probe fails & not --force-ebpf"| FALLBACK --> ST
  EB -->|"probe fails & --force-ebpf"| FORCE
```
