#!/usr/bin/env bash
set -euo pipefail

# Ensure bpf2go is reachable
# Declare GOPATH_BIN first to avoid masking `go env` return status
GOPATH_BIN="$(go env GOPATH)/bin"
export PATH="$GOPATH_BIN:$PATH"
export GOPACKAGE=tracer

# Find bpf2go in PATH without masking failures: declare then assign
BPF2GO=""
if BPF2GO_TMP=$(which bpf2go 2>/dev/null); then
  BPF2GO="$BPF2GO_TMP"
else
  echo "bpf2go not found in PATH; attempting 'go install' as best-effort"
  go install github.com/cilium/ebpf/cmd/bpf2go@latest || true
  if BPF2GO_TMP=$(which bpf2go 2>/dev/null); then
    BPF2GO="$BPF2GO_TMP"
  fi
fi

echo "GOPACKAGE=$GOPACKAGE"
echo "Using bpf2go: ${BPF2GO:-not found}"
echo "PWD: $(pwd)"

cd internal/tracer || { echo "internal/tracer not found"; exit 1; }

echo "Invoking: GOPACKAGE=$GOPACKAGE bpf2go -cc clang ebpf bpf/syscall.c"
# If a vmlinux.h was already generated in bpf/, prefer it and avoid
# pulling in system kernel headers which can conflict (redefinition errors).
if [ -f bpf/vmlinux.h ]; then
  CLANG_CFLAGS="-I./bpf -D__TARGET_ARCH_x86 -D__KERNEL__"
  echo "Found bpf/vmlinux.h; using minimal clang cflags: $CLANG_CFLAGS"
else
  # Provide explicit include paths and target arch define to clang invoked by bpf2go
  # This helps resolve kernel types like __u32/__u64 on runners that install headers
  CLANG_CFLAGS="-I/usr/include/x86_64-linux-gnu -I/usr/include -D__TARGET_ARCH_x86 -D__KERNEL__"

  # Try to detect installed kernel headers (common locations) and add include paths
  KHEADERS=""
  if [ -d "/usr/src/linux-headers-$(uname -r)/include" ]; then
    KHEADERS="/usr/src/linux-headers-$(uname -r)/include"
  else
    # fallback: pick the first matching linux-headers dir (use find to handle
    # arbitrary filenames safely instead of parsing ls output)
    # Find a linux-headers directory (declare then assign to avoid masking failures)
    KDIR=""
    if KDIR_TMP=$(find /usr/src -maxdepth 1 -type d -name 'linux-headers-*' 2>/dev/null | head -n1); then
      KDIR="$KDIR_TMP"
    fi
      if [ -n "$KDIR" ] && [ -d "$KDIR/include" ]; then
        KHEADERS="$KDIR/include"
      fi
  fi

  if [ -n "$KHEADERS" ]; then
    CLANG_CFLAGS="$CLANG_CFLAGS -I$KHEADERS -I$KHEADERS/uapi -I$KHEADERS/generated -I$KHEADERS/asm"
    echo "Detected kernel headers: $KHEADERS"
  fi
  # Ensure linux types are included so __u32/__u64 typedefs are available
  if [ -n "$KHEADERS" ] && [ -f "$KHEADERS/linux/types.h" ]; then
    CLANG_CFLAGS="$CLANG_CFLAGS -include $KHEADERS/linux/types.h"
  elif [ -f "/usr/include/linux/types.h" ]; then
    CLANG_CFLAGS="$CLANG_CFLAGS -include /usr/include/linux/types.h"
  fi

  echo "Using clang cflags: $CLANG_CFLAGS"
fi

GOPACKAGE=$GOPACKAGE bpf2go -cc clang -cflags "$CLANG_CFLAGS" ebpf bpf/syscall.c

echo "-- generated files in internal/tracer --"
ls -la . || true

if [ ! -f ebpf_bpfel.o ] || [ ! -f ebpf_bpfeb.o ]; then
  echo "ERROR: expected BPF .o files missing after bpf2go"
  ls -la . || true
  exit 1
fi
