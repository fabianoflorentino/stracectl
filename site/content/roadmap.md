---
title: "Roadmap"
description: "Planned features and known technical work for stracectl."
---

This page tracks planned features and known technical debt.
See also the [full Roadmap on GitHub](https://github.com/fabianoflorentino/stracectl/blob/main/docs/ROADMAP.md).

---

## Planned Features

### Direct `ptrace` backend

**Goal:** Remove the dependency on the `strace` binary by calling `ptrace(2)` directly from Go.

**Why:** Eliminates the runtime requirement of having `strace` installed on the host; opens the door to structured event metadata that the text-based parser cannot provide.

**Approach:**

- Use `golang.org/x/sys/unix` to call `ptrace(PTRACE_ATTACH, pid, ...)` in a dedicated goroutine locked to its OS thread (`runtime.LockOSThread`)
- Read registers with `PTRACE_GETREGS` on `SIGTRAP` to capture syscall number and arguments
- Replace `tracer/strace.go` with a new `tracer/ptrace.go` implementing the same `<-chan models.SyscallEvent` interface
- Keep the strace-subprocess tracer behind a `--backend strace` flag for fallback

---

### eBPF backend via `cilium/ebpf`

**Goal:** Zero-overhead syscall tracing without `ptrace`, suitable for production environments.

**Why:** `ptrace` serialises the traced process on every syscall entry/exit (significant overhead for high-rate processes); eBPF tracepoints run in kernel context with negligible overhead.

**Approach:**

- Attach to `raw_tracepoint/sys_enter` and `raw_tracepoint/sys_exit` using `cilium/ebpf`
- Share a `BPF_MAP_TYPE_RINGBUF` ring buffer between the BPF program and user-space reader
- Required capabilities: `CAP_BPF` + `CAP_PERFMON` (Linux 5.8+) — less privileged than `CAP_SYS_PTRACE`
- Add `--backend ebpf` flag; make it the default when the kernel supports it

---

### Fix `<unfinished ...>` line merging in the parser

**Goal:** Preserve argument data for blocking syscalls in multi-threaded processes.

**Current state:** `<unfinished ...>` lines are discarded. `Count` and `Latency` are accurate, but `Args` from the opening (unfinished) line are lost.

**Remaining work:**

- Add a `pendingLines map[int]string` keyed by PID to `Parser`
- On `<unfinished ...>`: store the partial line, return `nil, nil`
- On `<... resumed>`: splice prefix + suffix, delete the entry, resume normal parsing
- Update `parser_test.go` with multi-thread fixture cases
