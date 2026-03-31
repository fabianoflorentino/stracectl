#!/usr/bin/env bash
set -euo pipefail

# Ensure bpf2go is reachable
export PATH="$(go env GOPATH)/bin:$PATH"
export GOPACKAGE=tracer

echo "GOPACKAGE=$GOPACKAGE"
echo "Using bpf2go: $(which bpf2go || true)"
echo "PWD: $(pwd)"

cd internal/tracer || { echo "internal/tracer not found"; exit 1; }

echo "Invoking: GOPACKAGE=$GOPACKAGE bpf2go -cc clang ebpf bpf/syscall.c"
# Provide explicit include paths and target arch define to clang invoked by bpf2go
# This helps resolve kernel types like __u32/__u64 on some runners.
CLANG_CFLAGS="-I/usr/include/x86_64-linux-gnu -I/usr/include -D__TARGET_ARCH_x86"
echo "Using clang cflags: $CLANG_CFLAGS"
GOPACKAGE=$GOPACKAGE bpf2go -cc clang -cflags "$CLANG_CFLAGS" ebpf bpf/syscall.c

echo "-- generated files in internal/tracer --"
ls -la . || true

if [ ! -f ebpf_bpfel.o ] || [ ! -f ebpf_bpfeb.o ]; then
  echo "ERROR: expected BPF .o files missing after bpf2go"
  ls -la . || true
  exit 1
fi
