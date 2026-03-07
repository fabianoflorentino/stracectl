// Package tracer wraps the system strace binary to produce a stream of SyscallEvent values.
package tracer

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/parser"
)

// StraceTracer spawns a strace subprocess and emits parsed events on a channel.
type StraceTracer struct{}

func NewStraceTracer() *StraceTracer { return &StraceTracer{} }

// Attach attaches to a running process by PID.
// The caller must have sufficient privileges (CAP_SYS_PTRACE or ptrace scope 0).
func (t *StraceTracer) Attach(ctx context.Context, pid int) (<-chan models.SyscallEvent, error) {
	if err := checkStrace(); err != nil {
		return nil, err
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
		defer cmd.Wait() //nolint:errcheck

		scanner := bufio.NewScanner(stderr)
		// Increase buffer for lines with large read/write buffers in args.
		scanner.Buffer(make([]byte, 512*1024), 512*1024)

		for scanner.Scan() {
			event, err := parser.Parse(scanner.Text(), defaultPID)
			if err != nil || event == nil {
				continue
			}
			ch <- *event
		}
	}()

	return ch, nil
}

func checkStrace() error {
	if _, err := exec.LookPath("strace"); err != nil {
		return fmt.Errorf("strace not found in PATH — install it with: apt install strace")
	}
	return nil
}
