// stracectl is a modern strace replacement with real-time aggregation,
// per-syscall latency stats, and an interactive htop-style TUI.
//
// It supports attaching to running processes, tracing new commands,
// and exposing syscall metrics via an HTTP API in sidecar mode (--serve).
//
// Usage:
//
//	stracectl attach <pid>           # attach to a running process
//	stracectl run <cmd> [args...]    # trace a command from start
//	stracectl discover               # list traceable processes
//	stracectl trace --serve          # run in sidecar/HTTP mode
package main

import "github.com/fabianoflorentino/stracectl/cmd"

func main() {
	cmd.Execute()
}
