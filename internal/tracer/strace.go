// Package tracer wraps the system strace binary to produce a stream of SyscallEvent values.
package tracer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"syscall"

	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/parser"
)

// Tracer is the interface implemented by all tracing backends (strace, ptrace, eBPF).
// Both methods return a channel that emits events until the traced target exits or
// the context is cancelled, at which point the channel is closed.
type Tracer interface {
	// Attach attaches to an already-running process by its PID.
	Attach(ctx context.Context, pid int) (<-chan models.SyscallEvent, error)
	// Run launches program with the given args under the tracer.
	Run(ctx context.Context, program string, args []string) (<-chan models.SyscallEvent, error)
}

// StraceTracer spawns a strace subprocess and emits parsed events on a channel.
type StraceTracer struct{}

// NewStraceTracer creates a new StraceTracer instance.
func NewStraceTracer() *StraceTracer { return &StraceTracer{} }

// Attach attaches to a running process by PID.
// The caller must have sufficient privileges (CAP_SYS_PTRACE or ptrace scope 0).
func (t *StraceTracer) Attach(ctx context.Context, pid int) (<-chan models.SyscallEvent, error) {
	if err := checkStrace(); err != nil {
		return nil, err
	}
	if err := syscall.Kill(pid, 0); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil, fmt.Errorf("no process found with PID \033[1m%d\033[0m", pid)
		}

		return nil, fmt.Errorf("cannot access process \033[1m%d\033[0m: %w", pid, err)
	}

	cmd := exec.CommandContext(ctx, "strace", "-f", "-T", "-q", "-p", strconv.Itoa(pid))

	return t.start(cmd, pid)
}

// Run executes program with args under strace.
func (t *StraceTracer) Run(ctx context.Context, program string, args []string) (<-chan models.SyscallEvent, error) {
	if err := checkStrace(); err != nil {
		return nil, err
	}

	straceArgs := append([]string{"-f", "-T", "-q", "--", program}, args...)
	cmd := exec.CommandContext(ctx, "strace", straceArgs...)

	// Put strace and its subprocess into a separate process group so that
	// on context cancellation (user pressing q or Ctrl-C) the entire group
	// is killed atomically. Without this, the traced child (e.g. a long-running
	// "ping") survives strace being SIGKILL'd, keeping the stderr pipe open
	// and making the terminal appear frozen while wg.Wait() blocks.
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	if setpgid := reflect.ValueOf(cmd.SysProcAttr).Elem().FieldByName("Setpgid"); setpgid.IsValid() && setpgid.CanSet() && setpgid.Kind() == reflect.Bool {
		setpgid.SetBool(true)
	}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}

		// When Setpgid is available and true, the PGID equals strace's PID,
		// so Kill(-pid) reaches strace and all children. On targets where
		// Setpgid does not exist, fall back to killing only the tracer process.
		killPID := cmd.Process.Pid
		if cmd.SysProcAttr != nil {
			if setpgid := reflect.ValueOf(cmd.SysProcAttr).Elem().FieldByName("Setpgid"); setpgid.IsValid() && setpgid.Kind() == reflect.Bool && setpgid.Bool() {
				killPID = -killPID
			}
		}

		_ = syscall.Kill(killPID, syscall.SIGKILL)
		return nil
	}

	return t.start(cmd, 0)
}

func (t *StraceTracer) start(cmd *exec.Cmd, defaultPID int) (<-chan models.SyscallEvent, error) {
	// strace writes its trace to stderr; the traced program's own stderr is also mixed in,
	// but strace lines are unambiguous because of the syscall(args) = retval format.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start strace: %w", err)
	}

	ch := make(chan models.SyscallEvent, 4096)

	go func() {
		defer close(ch)

		var straceErrors []string

		defer func() {
			if err := cmd.Wait(); err != nil {
				// A killed-by-signal exit is expected when the context is
				// cancelled (normal quit). Only log genuine unexpected errors.
				var exitErr *exec.ExitError

				if errors.As(err, &exitErr) && exitErr.ExitCode() == -1 {
					return // killed by signal — context cancellation, not an error
				}

				if len(straceErrors) > 0 {
					log.Printf("strace: %s", strings.Join(straceErrors, "; "))
				} else {
					log.Printf("strace exited with error: %v", err)
				}
			}
		}()

		scanner := bufio.NewScanner(stderr)
		// Increase buffer for lines with large read/write buffers in args.
		scanner.Buffer(make([]byte, 512*1024), 512*1024)

		straceParser := parser.New()

		for scanner.Scan() {
			line := scanner.Text()
			event, err := straceParser.Parse(line, defaultPID)
			if err != nil {
				log.Printf("parse error: %v", err)
				continue
			}

			if event == nil {
				// Capture diagnostic lines emitted by strace itself (e.g. permission
				// errors or "No such process") so they can be shown if strace exits
				// with a non-zero code.
				if strings.HasPrefix(line, "strace:") {
					straceErrors = append(straceErrors, strings.TrimPrefix(strings.TrimSpace(line), "strace: "))
				}

				continue
			}
			// Debug: if the syscall failed with EAGAIN but has empty args,
			// log the raw strace line to aid diagnosing parser edge cases.
			if event.IsError() && event.Error == "EAGAIN" && event.Args == "" {
				// Only log this noisy diagnostic when the operator explicitly enables
				// verbose debug output via the STRACECTL_DEBUG environment variable.
				if os.Getenv("STRACECTL_DEBUG") == "1" {
					log.Printf("debug: EAGAIN with empty args — raw strace line: %q", line)
				}
			}
			ch <- *event
		}
		if err := scanner.Err(); err != nil {
			log.Printf("strace output read error: %v", err)
		}
	}()

	return ch, nil
}

// checkStrace verifies that the strace binary is available in PATH.
func checkStrace() error {
	if _, err := exec.LookPath("strace"); err != nil {
		return fmt.Errorf("strace not found in PATH — install it with: apt install strace")
	}

	return nil
}
