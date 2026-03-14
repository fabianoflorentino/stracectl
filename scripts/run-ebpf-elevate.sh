#!/usr/bin/env bash
set -euo pipefail

# Helper to run stracectl with elevated memlock so eBPF can be loaded.
# Usage: ./scripts/run-ebpf-elevate.sh -- <command> [args...]
# Example: ./scripts/run-ebpf-elevate.sh -- ls /

if [ "$#" -lt 1 ]; then
  echo "Usage: $0 -- <command> [args...]"
  exit 2
fi

# Find binary; prefer workspace bin/stracectl if available
EXE="./bin/stracectl"
if [ ! -x "$EXE" ]; then
  echo "Binary $EXE not found; building with ebpf tag..."
  CGO_ENABLED=1 go build -tags=ebpf -o bin/stracectl .
fi

# Shift until the -- separator (allow flags before -- in future)
if [ "$1" = "--" ]; then
  shift
fi

# Run with sudo + prlimit to set RLIMIT_MEMLOCK. This will prompt for your sudo password.
exec sudo prlimit --memlock=unlimited -- "$EXE" run --backend ebpf "$@"
