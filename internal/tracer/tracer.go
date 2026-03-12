package tracer

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/unix"
)

// unameFunc allows tests to override the uname syscall to simulate different
// kernel release strings. It defaults to unix.Uname and can be swapped in
// tests to avoid depending on the real kernel version.
var unameFunc = unix.Uname

// Select returns the appropriate Tracer implementation for the given backend name.
//
// Valid values: "auto", "ebpf", "strace".
// "auto" picks eBPF when the kernel supports it (Linux 5.8+), falling back to strace.
func Select(backend string) (Tracer, error) {
	switch backend {
	case "ebpf":
		return NewEBPFTracer(), nil
	case "strace":
		return NewStraceTracer(), nil
	case "auto", "":
		if ebpfAvailable() {
			return NewEBPFTracer(), nil
		}
		return NewStraceTracer(), nil
	default:
		return nil, fmt.Errorf("unknown backend %q: valid options are auto, ebpf, strace", backend)
	}
}

func ebpfAvailable() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// If this binary was not built with eBPF support, don't advertise it
	// as available even if the kernel version would otherwise support it.
	if !ebpfBuild {
		return false
	}

	var uname unix.Utsname
	if err := unameFunc(&uname); err != nil {
		return false
	}

	var major, minor int
	release := unameStr(uname.Release[:])
	if _, err := fmt.Sscanf(release, "%d.%d", &major, &minor); err != nil {
		return false
	}

	// BPF_MAP_TYPE_RINGBUF foi introduzido no Linux 5.8
	return major > 5 || (major == 5 && minor >= 8)
}
