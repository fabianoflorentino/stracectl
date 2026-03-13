// stracectl is a modern strace replacement with real-time aggregation,
// per-syscall latency stats, and an interactive htop-style TUI.
//
// It supports attaching to running processes, tracing new commands,
// and exposing syscall metrics via an HTTP API in sidecar mode (--serve).
//
// Usage:
//
//		stracectl run curl https://example.com           # trace a command from the start
//	  stracectl run --report out.html curl google.com  # trace and save an HTML report
//		stracectl attach 1234                            # attach to a running process
//		stracectl attach --serve :8080 1234              # attach and expose HTTP/Prometheus
//	  stracectl attach --backend ebpf 1234             # attach using eBPF (Linux 5.8+)
//	  stracectl attach --backend strace 1234           # attach using the classic strace subprocess tracer
//		stracectl stats trace.log                        # analyse a saved strace file
//		stracectl stats --serve :8080 trace.log          # serve stats from a saved file
//		stracectl stats --report report.html trace.log   # analyse and export an HTML report
//		stracectl discover myapp                         # find container PID in a Pod
package main

import "github.com/fabianoflorentino/stracectl/cmd"

func main() {
	cmd.Execute()
}
