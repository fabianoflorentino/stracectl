//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang ebpf bpf/syscall.c

//go:build ebpf
// +build ebpf

package tracer

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/cilium/ebpf/rlimit"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

// ebpfBuild is true when this package is compiled with the `ebpf` build tag.
// It is defined here (true) and in ebpf_stub.go (false) so runtime code can
// detect whether the binary was built with eBPF support.
var ebpfBuild = true

// ebpfEvent must exactly mirror the C struct above
// ebpfEvent must exactly mirror the C struct above
// NOTE: keep Path reasonably small to avoid blowing up ringbuf event size.
type ebpfEvent struct {
	PID       uint32
	SyscallNr uint32
	Ret       int64
	EnterNs   uint64
	ExitNs    uint64
	Args      [6]uint64
	Path      [128]byte
}

// EBPFTracer traces syscalls via eBPF without a subprocess.
type EBPFTracer struct {
	// Force, when true, causes eBPF probe failures to return an error
	// instead of transparently falling back to the strace tracer.
	Force bool
	// Unfiltered, when true, disables writing the traced process's PGID
	// into the BPF `root_pgid` filter so the BPF program captures all
	// processes (useful for environments where task->signal access may
	// fail, e.g. some WSL kernels).
	Unfiltered bool
}

func NewEBPFTracer() *EBPFTracer { return &EBPFTracer{} }

// SetForce configures the tracer to either fail-fast on eBPF probe errors
// (when true) or to fall back to the strace tracer (when false).
func (t *EBPFTracer) SetForce(v bool) { t.Force = v }

// SetUnfiltered toggles whether the tracer should write the traced PID's
// PGID into the BPF `root_pgid` filter. When true, the tracer skips writing
// the PGID to capture system-wide events.
func (t *EBPFTracer) SetUnfiltered(v bool) { t.Unfiltered = v }

func (t *EBPFTracer) Attach(ctx context.Context, pid int) (<-chan models.SyscallEvent, error) {
	// Probe whether eBPF can be loaded.
	if err := canLoadEbpf(); err != nil {
		if t.Force {
			return nil, fmt.Errorf("eBPF probe failed: %w", err)
		}
		log.Printf("eBPF probe failed: %v; falling back to strace tracer", err)
		return NewStraceTracer().Attach(ctx, pid)
	}

	return t.trace(ctx, pid)
}

func (t *EBPFTracer) Run(ctx context.Context, program string, args []string) (<-chan models.SyscallEvent, error) {
	// Probe whether eBPF can be loaded before starting the traced process.
	// If the probe fails, transparently fall back to the strace subprocess
	// tracer which does not require loading BPF programs.
	if err := canLoadEbpf(); err != nil {
		if t.Force {
			return nil, fmt.Errorf("eBPF probe failed: %w", err)
		}
		log.Printf("eBPF probe failed: %v; falling back to strace tracer", err)
		return NewStraceTracer().Run(ctx, program, args)
	}

	// Start the traced program and attach the eBPF tracer to its PID.
	// Mirror the process-group handling used by the Strace tracer so the
	// entire traced process group is killed on cancellation.
	cmd := exec.CommandContext(ctx, program, args...)

	cmd.SysProcAttr = &syscall.SysProcAttr{}
	if setpgid := reflect.ValueOf(cmd.SysProcAttr).Elem().FieldByName("Setpgid"); setpgid.IsValid() && setpgid.CanSet() && setpgid.Kind() == reflect.Bool {
		setpgid.SetBool(true)
	}

	// Override Cancel to kill the process group when available.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}

		killPID := cmd.Process.Pid
		if cmd.SysProcAttr != nil {
			if setpgid := reflect.ValueOf(cmd.SysProcAttr).Elem().FieldByName("Setpgid"); setpgid.IsValid() && setpgid.Kind() == reflect.Bool && setpgid.Bool() {
				killPID = -killPID
			}
		}

		_ = syscall.Kill(killPID, syscall.SIGKILL)
		return nil
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start traced process: %w", err)
	}

	pid := cmd.Process.Pid

	// Pause the traced process immediately so we can attach BPF programs and
	// set the filtering map before the process issues syscalls. This avoids a
	// race where a short-lived process finishes before the eBPF probe is
	// installed and results in zero captured events.
	if err := syscall.Kill(pid, syscall.SIGSTOP); err != nil {
		log.Printf("warning: failed to SIGSTOP pid %d: %v", pid, err)
	} else {
		log.Printf("ebpf: paused traced process pid=%d for attach", pid)
	}

	// tracerCtx is cancelled when the traced process exits or the parent ctx
	// is cancelled. When tracerCtx is cancelled the eBPF trace goroutine will
	// exit (see t.trace which listens on the context).
	tracerCtx, cancel := context.WithCancel(ctx)

	go func() {
		// When the traced process exits, cancel the tracer context to stop
		// the eBPF reader goroutine.
		_ = cmd.Wait()
		cancel()
	}()

	events, err := t.trace(tracerCtx, pid)
	if err != nil {
		// If attaching/tracing failed, ensure the traced process is killed.
		_ = cmd.Cancel()
		return nil, err
	}

	// Resume the traced process now that the eBPF programs and filters are in
	// place. If SIGCONT fails, log and continue; the process may already have
	// exited.
	if err := syscall.Kill(pid, syscall.SIGCONT); err != nil {
		log.Printf("warning: failed to SIGCONT pid %d: %v", pid, err)
	} else {
		log.Printf("ebpf: resumed traced process pid=%d after attach", pid)
	}

	return events, nil
}

// canLoadEbpf attempts to raise RLIMIT_MEMLOCK (best-effort) and load the
// embedded BPF objects. It returns nil when loading succeeds; callers should
// still call the normal load path later (this is only a runtime probe used
// to decide whether to fall back to strace).
func canLoadEbpf() error {
	// Best-effort raise memlock; don't treat failure as fatal yet.
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Printf("warning: failed to raise RLIMIT_MEMLOCK: %v", err)
	}

	objs := ebpfObjects{}
	if err := loadEbpfObjects(&objs, nil); err != nil {
		return err
	}
	_ = objs.Close()
	log.Printf("canLoadEbpf: loaded embedded BPF objects successfully")
	return nil
}

// putRootPgid locates the root_pgid map within the generated ebpfObjects
// (which may vary between generated files) and writes the provided value.
// This uses reflection to be resilient to minor differences in generated
// wrapper layouts across architectures or generator versions.
func putRootPgid(objs any, key uint32, val uint32) error {
	vo := reflect.ValueOf(objs)
	if vo.Kind() == reflect.Ptr {
		vo = vo.Elem()
	}

	// Direct promoted field (objs.RootPgid)
	if f := vo.FieldByName("RootPgid"); f.IsValid() && f.CanInterface() {
		if mp, ok := f.Interface().(*ebpf.Map); ok && mp != nil {
			return mp.Put(key, val)
		}
	}

	// Nested ebpfMaps field (objs.ebpfMaps.RootPgid)
	if m := vo.FieldByName("ebpfMaps"); m.IsValid() {
		mf := m
		if mf.Kind() == reflect.Ptr && !mf.IsNil() {
			mf = mf.Elem()
		}
		if rf := mf.FieldByName("RootPgid"); rf.IsValid() && rf.CanInterface() {
			if mp, ok := rf.Interface().(*ebpf.Map); ok && mp != nil {
				return mp.Put(key, val)
			}
		}
	}

	// Fallback: search for struct/tag named root_pgid
	t := vo.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Tag.Get("ebpf") == "root_pgid" {
			fv := vo.Field(i)
			if fv.IsValid() && fv.CanInterface() {
				if mp, ok := fv.Interface().(*ebpf.Map); ok && mp != nil {
					return mp.Put(key, val)
				}
			}
		}
		// check nested struct fields
		fv := vo.Field(i)
		if fv.Kind() == reflect.Struct {
			ft := fv.Type()
			for j := 0; j < ft.NumField(); j++ {
				if ft.Field(j).Tag.Get("ebpf") == "root_pgid" {
					rf := fv.Field(j)
					if rf.IsValid() && rf.CanInterface() {
						if mp, ok := rf.Interface().(*ebpf.Map); ok && mp != nil {
							return mp.Put(key, val)
						}
					}
				}
			}
		}
	}

	return fmt.Errorf("root_pgid map not found")
}

func (t *EBPFTracer) trace(ctx context.Context, filterPID int) (<-chan models.SyscallEvent, error) {
	// Best-effort: try raising RLIMIT_MEMLOCK so BPF maps/programs can be created.
	// If this fails (non-root), log a warning and continue; loadEbpfObjects
	// will likely fail with a helpful error if limits are insufficient.
	if err := rlimit.RemoveMemlock(); err != nil {
		log.Printf("warning: failed to raise RLIMIT_MEMLOCK: %v; you may need to run as root or use: sudo prlimit --memlock=unlimited ./stracectl ...", err)
	}

	objs := ebpfObjects{}
	if err := loadEbpfObjects(&objs, nil); err != nil {
		return nil, fmt.Errorf("load bpf objects: %w", err)
	}
	log.Printf("ebpf: loaded BPF objects OK")

	// When a root PID is given, write its process group ID (PGID) into
	// root_pgid[0]. The BPF program compares the current task's PGID to this
	// value, so writing the actual PGID (not the PID) ensures the filter
	// matches correctly even if Setpgid handling differs on some platforms.
	if filterPID > 0 {
		key := uint32(0)
		if t.Unfiltered {
			// Explicitly set 0 (unfiltered) when the operator requested
			// unfiltered mode.
			zero := uint32(0)
			if err := putRootPgid(&objs, key, zero); err != nil {
				log.Printf("warning: root_pgid put failed: %v; continuing unfiltered", err)
			} else {
				log.Printf("ebpf: root_pgid set to 0 (unfiltered)")
			}
		} else {
			// Attempt to write the traced process group ID (PGID) into the
			// BPF filter so the eBPF program only accepts events from the
			// traced process's process group. If retrieving or writing the
			// PGID fails, fall back to unfiltered mode (0) to avoid dropping
			// events silently.
			pgid, err := syscall.Getpgid(filterPID)
			if err != nil {
				log.Printf("warning: failed to get pgid for pid %d: %v; using unfiltered mode", filterPID, err)
				zero := uint32(0)
				if err := putRootPgid(&objs, key, zero); err != nil {
					log.Printf("warning: root_pgid put failed: %v; continuing unfiltered", err)
				} else {
					log.Printf("ebpf: root_pgid set to 0 (unfiltered)")
				}
			} else {
				val := uint32(pgid)
				if err := putRootPgid(&objs, key, val); err != nil {
					log.Printf("warning: root_pgid put failed: %v; falling back to unfiltered", err)
					zero := uint32(0)
					_ = putRootPgid(&objs, key, zero)
				} else {
					log.Printf("ebpf: root_pgid set to %d (pgid of pid %d)", val, filterPID)
				}
			}
		}
	}

	enterLink, err := link.AttachRawTracepoint(link.RawTracepointOptions{
		Name:    "sys_enter",
		Program: objs.SysEnter,
	})
	if err != nil {
		objs.Close()
		return nil, fmt.Errorf("attach sys_enter: %w", err)
	}
	log.Printf("ebpf: attached sys_enter tracepoint")

	exitLink, err := link.AttachRawTracepoint(link.RawTracepointOptions{
		Name:    "sys_exit",
		Program: objs.SysExit,
	})
	if err != nil {
		enterLink.Close()
		objs.Close()
		return nil, fmt.Errorf("attach sys_exit: %w", err)
	}
	log.Printf("ebpf: attached sys_exit tracepoint")

	rd, err := ringbuf.NewReader(objs.Events)
	if err != nil {
		enterLink.Close()
		exitLink.Close()
		objs.Close()
		return nil, fmt.Errorf("ringbuf reader: %w", err)
	}
	log.Printf("ebpf: ringbuf reader created")

	ch := make(chan models.SyscallEvent, 4096)

	go func() {
		defer close(ch)
		defer enterLink.Close()
		defer exitLink.Close()
		defer rd.Close()
		defer objs.Close()

		syscallNames := buildSyscallTable()
		evCount := 0

		for {
			select {
			case <-ctx.Done():
				log.Printf("ebpf: reader context done, exiting read loop (events read=%d)", evCount)
				return
			default:
			}

			record, err := rd.Read()
			if err != nil {
				log.Printf("ebpf: ringbuf read error: %v (events read=%d)", err, evCount)
				return
			}

			var raw ebpfEvent
			copy(unsafe.Slice((*byte)(unsafe.Pointer(&raw)), unsafe.Sizeof(raw)),
				record.RawSample)

			name := syscallNames[raw.SyscallNr]
			if name == "" {
				name = fmt.Sprintf("syscall_%d", raw.SyscallNr)
			}

			latency := time.Duration(raw.ExitNs - raw.EnterNs)
			argsStr := formatSyscallArgs(name, raw.Args, raw.Ret)
			// If the BPF event included a captured path, prefer that as the
			// first (quoted) argument so downstream path extraction can find it.
			if len(raw.Path) > 0 {
				// Trim trailing NUL bytes
				p := strings.TrimRight(string(raw.Path[:]), "\x00")
				// Reject suspicious or empty strings
				ok := true
				if p == "" {
					ok = false
				}
				for _, r := range p {
					if r == '\x00' || (r < 32 && r != '\t') {
						ok = false
						break
					}
				}
				if ok {
					// Quote the path so extractPathFromArgs can parse it as a quoted string.
					argsStr = strconv.Quote(p)
				}
			}

			retValStr := fmt.Sprintf("%d", raw.Ret)
			var errnoStr string
			if raw.Ret < 0 {
				retValStr = "-1"
				errnoStr = ErrnoName(int(-raw.Ret))
				if errnoStr == "" {
					errnoStr = fmt.Sprintf("ERR%d", -raw.Ret)
				}
			}

			evCount++
			if evCount == 1 {
				log.Printf("ebpf: first event pid=%d syscall=%s ret=%d", raw.PID, name, raw.Ret)
			}

			ch <- models.SyscallEvent{
				PID:     int(raw.PID),
				Name:    name,
				Args:    argsStr,
				RetVal:  retValStr,
				Error:   errnoStr,
				Latency: latency,
				Time:    time.Now(),
			}
		}
	}()

	return ch, nil
}

// formatSyscallArgs formats the raw argument array into a human-readable string
// that matches the strace output style as closely as possible.
//
// For well-known syscalls the arguments are labelled with their semantic meaning
// (e.g. fd, flags, mode). For unknown syscalls a generic hex/decimal heuristic
// is used so that the output is never empty.
func formatSyscallArgs(name string, args [6]uint64, ret int64) string {
	switch name {
	// ── file descriptors / paths ──────────────────────────────────────────
	case "read", "write":
		return fmt.Sprintf("%d, 0x%x, %d", args[0], args[1], args[2])
	case "pread64", "pwrite64":
		return fmt.Sprintf("%d, 0x%x, %d, %d", args[0], args[1], args[2], args[3])
	case "open":
		return fmt.Sprintf("0x%x, %s, %04o", args[0], openFlagsStr(args[1]), args[2])
	case "openat":
		return fmt.Sprintf("%s, 0x%x, %s, %04o", atFdStr(args[0]), args[1], openFlagsStr(args[2]), args[3])
	case "close":
		return fmt.Sprintf("%d", args[0])
	case "stat", "lstat", "fstat":
		return fmt.Sprintf("0x%x, 0x%x", args[0], args[1])
	case "newfstatat":
		return fmt.Sprintf("%s, 0x%x, 0x%x, %d", atFdStr(args[0]), args[1], args[2], args[3])
	case "lseek":
		return fmt.Sprintf("%d, %d, %d", args[0], int64(args[1]), args[2])
	case "dup", "dup2", "dup3":
		return fmt.Sprintf("%d, %d, %d", args[0], args[1], args[2])
	case "fcntl":
		return fmt.Sprintf("%d, %d, %d", args[0], args[1], args[2])
	case "ftruncate", "truncate":
		return fmt.Sprintf("%d, %d", args[0], args[1])
	case "unlink", "rmdir", "mkdir":
		return fmt.Sprintf("0x%x", args[0])
	case "unlinkat", "mkdirat":
		return fmt.Sprintf("%s, 0x%x, %d", atFdStr(args[0]), args[1], args[2])
	case "rename":
		return fmt.Sprintf("0x%x, 0x%x", args[0], args[1])
	case "renameat", "renameat2":
		return fmt.Sprintf("%s, 0x%x, %s, 0x%x", atFdStr(args[0]), args[1], atFdStr(args[2]), args[3])
	case "link", "symlink":
		return fmt.Sprintf("0x%x, 0x%x", args[0], args[1])
	case "linkat", "symlinkat":
		return fmt.Sprintf("%s, 0x%x, %s, 0x%x", atFdStr(args[0]), args[1], atFdStr(args[2]), args[3])
	case "readlink":
		return fmt.Sprintf("0x%x, 0x%x, %d", args[0], args[1], args[2])
	case "readlinkat":
		return fmt.Sprintf("%s, 0x%x, 0x%x, %d", atFdStr(args[0]), args[1], args[2], args[3])
	case "chmod", "chown", "lchown":
		return fmt.Sprintf("0x%x, %04o", args[0], args[1])
	case "fchmod":
		return fmt.Sprintf("%d, %04o", args[0], args[1])
	case "fchown", "fchownat":
		return fmt.Sprintf("%d, %d, %d", args[0], args[1], args[2])
	case "access", "faccessat":
		return fmt.Sprintf("0x%x, %d", args[0], args[1])
	case "chdir", "fchdir":
		return fmt.Sprintf("0x%x", args[0])
	case "getcwd":
		return fmt.Sprintf("0x%x, %d", args[0], args[1])
	case "getdents", "getdents64":
		return fmt.Sprintf("%d, 0x%x, %d", args[0], args[1], args[2])
	case "inotify_add_watch":
		return fmt.Sprintf("%d, 0x%x, 0x%x", args[0], args[1], args[2])
	case "inotify_rm_watch":
		return fmt.Sprintf("%d, %d", args[0], args[1])

	// ── memory ────────────────────────────────────────────────────────────
	case "mmap":
		return fmt.Sprintf("0x%x, %d, %s, %s, %d, %d",
			args[0], args[1], mmapProtStr(args[2]), mmapFlagsStr(args[3]),
			int32(args[4]), args[5])
	case "munmap":
		return fmt.Sprintf("0x%x, %d", args[0], args[1])
	case "mprotect":
		return fmt.Sprintf("0x%x, %d, %s", args[0], args[1], mmapProtStr(args[2]))
	case "mremap":
		return fmt.Sprintf("0x%x, %d, %d, %d", args[0], args[1], args[2], args[3])
	case "madvise":
		return fmt.Sprintf("0x%x, %d, %d", args[0], args[1], args[2])
	case "brk":
		return fmt.Sprintf("0x%x", args[0])

	// ── process / threading ───────────────────────────────────────────────
	case "clone":
		return fmt.Sprintf("%s, 0x%x, 0x%x, 0x%x, %d",
			cloneFlagsStr(args[0]), args[1], args[2], args[3], args[4])
	case "clone3":
		return fmt.Sprintf("0x%x, %d", args[0], args[1])
	case "fork", "vfork":
		return ""
	case "execve":
		return fmt.Sprintf("0x%x, 0x%x, 0x%x", args[0], args[1], args[2])
	case "execveat":
		return fmt.Sprintf("%s, 0x%x, 0x%x, 0x%x, %d",
			atFdStr(args[0]), args[1], args[2], args[3], args[4])
	case "exit", "exit_group":
		return fmt.Sprintf("%d", args[0])
	case "wait4":
		return fmt.Sprintf("%d, 0x%x, %d, 0x%x", int32(args[0]), args[1], args[2], args[3])
	case "waitid":
		return fmt.Sprintf("%d, %d, 0x%x, %d", args[0], args[1], args[2], args[3])
	case "kill":
		return fmt.Sprintf("%d, %d", int32(args[0]), args[1])
	case "tgkill", "tkill":
		return fmt.Sprintf("%d, %d, %d", args[0], args[1], args[2])
	case "getpid", "gettid", "getppid", "getuid", "geteuid", "getgid", "getegid":
		return ""
	case "setuid", "setgid", "setreuid", "setregid":
		return fmt.Sprintf("%d, %d", args[0], args[1])
	case "getpriority", "setpriority":
		return fmt.Sprintf("%d, %d, %d", args[0], args[1], int32(args[2]))
	case "prctl":
		return fmt.Sprintf("%d, %d, %d, %d, %d",
			args[0], args[1], args[2], args[3], args[4])
	case "arch_prctl":
		return fmt.Sprintf("%d, 0x%x", args[0], args[1])
	case "sched_yield":
		return ""
	case "sched_getaffinity", "sched_setaffinity":
		return fmt.Sprintf("%d, %d, 0x%x", args[0], args[1], args[2])
	case "nanosleep", "clock_nanosleep":
		return fmt.Sprintf("0x%x, 0x%x", args[0], args[1])
	case "getitimer", "setitimer":
		return fmt.Sprintf("%d, 0x%x, 0x%x", args[0], args[1], args[2])
	case "alarm":
		return fmt.Sprintf("%d", args[0])
	case "pause":
		return ""

	// ── signals ───────────────────────────────────────────────────────────
	case "rt_sigaction":
		return fmt.Sprintf("%d, 0x%x, 0x%x, %d", args[0], args[1], args[2], args[3])
	case "rt_sigprocmask":
		return fmt.Sprintf("%d, 0x%x, 0x%x, %d", args[0], args[1], args[2], args[3])
	case "rt_sigreturn":
		return ""
	case "rt_sigpending":
		return fmt.Sprintf("0x%x", args[0])
	case "rt_sigsuspend":
		return fmt.Sprintf("0x%x, %d", args[0], args[1])
	case "signalfd", "signalfd4":
		return fmt.Sprintf("%d, 0x%x, %d", int32(args[0]), args[1], args[2])
	case "sigaltstack":
		return fmt.Sprintf("0x%x, 0x%x", args[0], args[1])

	// ── networking ────────────────────────────────────────────────────────
	case "socket":
		return fmt.Sprintf("%d, %s, %d", args[0], socketTypeStr(args[1]), args[2])
	case "bind", "connect", "accept":
		return fmt.Sprintf("%d, 0x%x, %d", args[0], args[1], args[2])
	case "accept4":
		return fmt.Sprintf("%d, 0x%x, 0x%x, %d", args[0], args[1], args[2], args[3])
	case "listen":
		return fmt.Sprintf("%d, %d", args[0], args[1])
	case "send", "recv":
		return fmt.Sprintf("%d, 0x%x, %d, %d", args[0], args[1], args[2], args[3])
	case "sendto", "recvfrom":
		return fmt.Sprintf("%d, 0x%x, %d, %d, 0x%x, 0x%x",
			args[0], args[1], args[2], args[3], args[4], args[5])
	case "sendmsg", "recvmsg":
		return fmt.Sprintf("%d, 0x%x, %d", args[0], args[1], args[2])
	case "sendmmsg", "recvmmsg":
		return fmt.Sprintf("%d, 0x%x, %d, %d", args[0], args[1], args[2], args[3])
	case "getsockname", "getpeername":
		return fmt.Sprintf("%d, 0x%x, 0x%x", args[0], args[1], args[2])
	case "setsockopt", "getsockopt":
		return fmt.Sprintf("%d, %d, %d, 0x%x, %d",
			args[0], args[1], args[2], args[3], args[4])
	case "socketpair":
		return fmt.Sprintf("%d, %s, %d, 0x%x",
			args[0], socketTypeStr(args[1]), args[2], args[3])
	case "shutdown":
		return fmt.Sprintf("%d, %d", args[0], args[1])

	// ── I/O multiplexing / epoll / poll ───────────────────────────────────
	case "select", "_newselect":
		return fmt.Sprintf("%d, 0x%x, 0x%x, 0x%x, 0x%x",
			args[0], args[1], args[2], args[3], args[4])
	case "pselect6":
		return fmt.Sprintf("%d, 0x%x, 0x%x, 0x%x, 0x%x, 0x%x",
			args[0], args[1], args[2], args[3], args[4], args[5])
	case "poll", "ppoll":
		return fmt.Sprintf("0x%x, %d, %d", args[0], args[1], int32(args[2]))
	case "epoll_create", "epoll_create1":
		return fmt.Sprintf("%d", args[0])
	case "epoll_ctl":
		return fmt.Sprintf("%d, %d, %d, 0x%x", args[0], args[1], args[2], args[3])
	case "epoll_wait", "epoll_pwait":
		return fmt.Sprintf("%d, 0x%x, %d, %d", args[0], args[1], args[2], int32(args[3]))

	// ── file I/O advanced ─────────────────────────────────────────────────
	case "sendfile", "sendfile64":
		return fmt.Sprintf("%d, %d, 0x%x, %d", args[0], args[1], args[2], args[3])
	case "splice":
		return fmt.Sprintf("%d, 0x%x, %d, 0x%x, %d, %d",
			args[0], args[1], args[2], args[3], args[4], args[5])
	case "tee":
		return fmt.Sprintf("%d, %d, %d, %d", args[0], args[1], args[2], args[3])
	case "readv", "writev":
		return fmt.Sprintf("%d, 0x%x, %d", args[0], args[1], args[2])
	case "preadv", "pwritev", "preadv2", "pwritev2":
		return fmt.Sprintf("%d, 0x%x, %d, %d", args[0], args[1], args[2], int64(args[3]))
	case "copy_file_range":
		return fmt.Sprintf("%d, 0x%x, %d, 0x%x, %d, %d",
			args[0], args[1], args[2], args[3], args[4], args[5])
	case "fallocate":
		return fmt.Sprintf("%d, %d, %d, %d", args[0], args[1], int64(args[2]), int64(args[3]))
	case "sync", "sync_file_range":
		return fmt.Sprintf("%d", args[0])
	case "fsync", "fdatasync":
		return fmt.Sprintf("%d", args[0])
	case "syncfs":
		return fmt.Sprintf("%d", args[0])

	// ── pipes / eventfd / timerfd ─────────────────────────────────────────
	case "pipe", "pipe2":
		return fmt.Sprintf("0x%x, %d", args[0], args[1])
	case "eventfd", "eventfd2":
		return fmt.Sprintf("%d, %d", args[0], args[1])
	case "timerfd_create":
		return fmt.Sprintf("%d, %d", args[0], args[1])
	case "timerfd_settime":
		return fmt.Sprintf("%d, %d, 0x%x, 0x%x", args[0], args[1], args[2], args[3])
	case "timerfd_gettime":
		return fmt.Sprintf("%d, 0x%x", args[0], args[1])

	// ── system info ───────────────────────────────────────────────────────
	case "uname":
		return fmt.Sprintf("0x%x", args[0])
	case "sysinfo":
		return fmt.Sprintf("0x%x", args[0])
	case "times":
		return fmt.Sprintf("0x%x", args[0])
	case "getrlimit", "setrlimit", "prlimit64":
		return fmt.Sprintf("%d, 0x%x", args[0], args[1])
	case "getrusage":
		return fmt.Sprintf("%d, 0x%x", args[0], args[1])
	case "clock_gettime", "clock_settime", "clock_getres":
		return fmt.Sprintf("%d, 0x%x", args[0], args[1])
	case "gettimeofday", "settimeofday":
		return fmt.Sprintf("0x%x, 0x%x", args[0], args[1])
	case "time":
		return fmt.Sprintf("0x%x", args[0])

	// ── ioctl / misc ──────────────────────────────────────────────────────
	case "ioctl":
		return fmt.Sprintf("%d, 0x%x, 0x%x", args[0], args[1], args[2])
	case "futex":
		return fmt.Sprintf("0x%x, %d, %d, 0x%x, 0x%x, %d",
			args[0], args[1], int32(args[2]), args[3], args[4], int32(args[5]))
	case "set_robust_list", "get_robust_list":
		return fmt.Sprintf("0x%x, %d", args[0], args[1])
	case "mlock", "munlock", "mlock2":
		return fmt.Sprintf("0x%x, %d", args[0], args[1])
	case "mlockall", "munlockall":
		return fmt.Sprintf("%d", args[0])
	case "mincore":
		return fmt.Sprintf("0x%x, %d, 0x%x", args[0], args[1], args[2])
	case "msync":
		return fmt.Sprintf("0x%x, %d, %d", args[0], args[1], args[2])
	case "syslog":
		return fmt.Sprintf("%d, 0x%x, %d", args[0], args[1], args[2])
	case "ptrace":
		return fmt.Sprintf("%d, %d, 0x%x, 0x%x", args[0], args[1], args[2], args[3])
	case "perf_event_open":
		return fmt.Sprintf("0x%x, %d, %d, %d, %d",
			args[0], int32(args[1]), int32(args[2]), int32(args[3]), args[4])
	case "bpf":
		return fmt.Sprintf("%d, 0x%x, %d", args[0], args[1], args[2])
	case "seccomp":
		return fmt.Sprintf("%d, %d, 0x%x", args[0], args[1], args[2])
	case "getrandom":
		return fmt.Sprintf("0x%x, %d, %d", args[0], args[1], args[2])
	case "memfd_create":
		return fmt.Sprintf("0x%x, %d", args[0], args[1])
	case "statfs", "fstatfs":
		return fmt.Sprintf("0x%x, 0x%x", args[0], args[1])
	case "mount":
		return fmt.Sprintf("0x%x, 0x%x, 0x%x, %d, 0x%x",
			args[0], args[1], args[2], args[3], args[4])
	case "umount2":
		return fmt.Sprintf("0x%x, %d", args[0], args[1])
	case "pivot_root":
		return fmt.Sprintf("0x%x, 0x%x", args[0], args[1])
	case "chroot":
		return fmt.Sprintf("0x%x", args[0])
	case "mknod", "mknodat":
		return fmt.Sprintf("0x%x, %04o, %d", args[0], args[1], args[2])
	case "utimes", "utimensat":
		return fmt.Sprintf("0x%x, 0x%x", args[0], args[1])

	default:
		// Generic fallback: show non-zero args in hex/decimal heuristic.
		return formatGenericArgs(args)
	}
}

// formatGenericArgs is the fallback formatter used for syscalls not explicitly
// handled above. It reproduces the original heuristic (hex for large/pointer
// values, decimal for small integers) but skips trailing zero arguments to
// keep output concise, matching strace's behaviour for unused parameters.
func formatGenericArgs(args [6]uint64) string {
	// Find last non-zero arg to avoid printing a long tail of zeroes.
	last := -1
	for i := 5; i >= 0; i-- {
		if args[i] != 0 {
			last = i
			break
		}
	}
	if last < 0 {
		return ""
	}

	parts := make([]string, last+1)
	for i := 0; i <= last; i++ {
		parts[i] = formatRawArg(args[i])
	}
	return strings.Join(parts, ", ")
}

// formatRawArg formats a single raw uint64 argument value.
// Values that look like pointers or are large are shown in hex; small integers
// in decimal — matching the heuristic that strace uses when it cannot decode
// the argument type.
func formatRawArg(v uint64) string {
	if v == 0 {
		return "0"
	}
	if v > 0xffff {
		return fmt.Sprintf("0x%x", v)
	}
	return fmt.Sprintf("%d", v)
}

// ── flag / constant helpers ──────────────────────────────────────────────────

func openFlagsStr(flags uint64) string {
	const (
		O_RDONLY   = 0
		O_WRONLY   = 1
		O_RDWR     = 2
		O_CREAT    = 0100
		O_TRUNC    = 01000
		O_APPEND   = 02000
		O_NONBLOCK = 04000
		O_CLOEXEC  = 02000000
	)
	if flags == O_RDONLY {
		return "O_RDONLY"
	}
	var parts []string
	mode := flags & 3
	switch mode {
	case O_WRONLY:
		parts = append(parts, "O_WRONLY")
	case O_RDWR:
		parts = append(parts, "O_RDWR")
	default:
		parts = append(parts, "O_RDONLY")
	}
	if flags&O_CREAT != 0 {
		parts = append(parts, "O_CREAT")
	}
	if flags&O_TRUNC != 0 {
		parts = append(parts, "O_TRUNC")
	}
	if flags&O_APPEND != 0 {
		parts = append(parts, "O_APPEND")
	}
	if flags&O_NONBLOCK != 0 {
		parts = append(parts, "O_NONBLOCK")
	}
	if flags&O_CLOEXEC != 0 {
		parts = append(parts, "O_CLOEXEC")
	}
	if len(parts) == 0 {
		return fmt.Sprintf("0x%x", flags)
	}
	return strings.Join(parts, "|")
}

func atFdStr(fd uint64) string {
	const AT_FDCWD = ^uint64(99) // 0xffffffffffffff9c == -100 as uint64
	if fd == AT_FDCWD {
		return "AT_FDCWD"
	}
	return fmt.Sprintf("%d", int32(fd))
}

func mmapProtStr(prot uint64) string {
	if prot == 0 {
		return "PROT_NONE"
	}
	var parts []string
	if prot&1 != 0 {
		parts = append(parts, "PROT_READ")
	}
	if prot&2 != 0 {
		parts = append(parts, "PROT_WRITE")
	}
	if prot&4 != 0 {
		parts = append(parts, "PROT_EXEC")
	}
	if len(parts) == 0 {
		return fmt.Sprintf("0x%x", prot)
	}
	return strings.Join(parts, "|")
}

func mmapFlagsStr(flags uint64) string {
	var parts []string
	switch flags & 0xf {
	case 1:
		parts = append(parts, "MAP_SHARED")
	case 2:
		parts = append(parts, "MAP_PRIVATE")
	default:
		parts = append(parts, fmt.Sprintf("MAP_0x%x", flags&0xf))
	}
	if flags&0x20 != 0 {
		parts = append(parts, "MAP_ANONYMOUS")
	}
	if flags&0x10 != 0 {
		parts = append(parts, "MAP_FIXED")
	}
	if flags&0x100 != 0 {
		parts = append(parts, "MAP_GROWSDOWN")
	}
	if flags&0x800 != 0 {
		parts = append(parts, "MAP_NORESERVE")
	}
	if flags&0x1000 != 0 {
		parts = append(parts, "MAP_LOCKED")
	}
	if flags&0x4000 != 0 {
		parts = append(parts, "MAP_POPULATE")
	}
	return strings.Join(parts, "|")
}

func socketTypeStr(typ uint64) string {
	base := typ &^ 0xf00 // strip SOCK_NONBLOCK / SOCK_CLOEXEC
	var name string
	switch base {
	case 1:
		name = "SOCK_STREAM"
	case 2:
		name = "SOCK_DGRAM"
	case 3:
		name = "SOCK_RAW"
	case 5:
		name = "SOCK_SEQPACKET"
	default:
		name = fmt.Sprintf("SOCK_0x%x", base)
	}
	var extras []string
	if typ&0x800 != 0 {
		extras = append(extras, "SOCK_NONBLOCK")
	}
	if typ&0x80000 != 0 {
		extras = append(extras, "SOCK_CLOEXEC")
	}
	if len(extras) > 0 {
		return name + "|" + strings.Join(extras, "|")
	}
	return name
}

func cloneFlagsStr(flags uint64) string {
	known := map[uint64]string{
		0x00000100: "CLONE_VM",
		0x00000200: "CLONE_FS",
		0x00000400: "CLONE_FILES",
		0x00000800: "CLONE_SIGHAND",
		0x00002000: "CLONE_PTRACE",
		0x00004000: "CLONE_VFORK",
		0x00008000: "CLONE_PARENT",
		0x00010000: "CLONE_THREAD",
		0x00020000: "CLONE_NEWNS",
		0x00080000: "CLONE_SYSVSEM",
		0x00100000: "CLONE_SETTLS",
		0x00200000: "CLONE_PARENT_SETTID",
		0x00400000: "CLONE_CHILD_CLEARTID",
		0x01000000: "CLONE_UNTRACED",
		0x02000000: "CLONE_CHILD_SETTID",
		0x10000000: "CLONE_NEWUTS",
		0x20000000: "CLONE_NEWIPC",
		0x40000000: "CLONE_NEWUSER",
		0x80000000: "CLONE_NEWPID",
	}
	var parts []string
	for bit, name := range known {
		if flags&bit != 0 {
			parts = append(parts, name)
		}
	}
	// Signal number in lowest byte
	if sig := flags & 0xff; sig != 0 {
		parts = append(parts, fmt.Sprintf("sig=%d", sig))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("0x%x", flags)
	}
	return strings.Join(parts, "|")
}
