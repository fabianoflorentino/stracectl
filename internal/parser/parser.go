// Package parser parses strace output lines into SyscallEvent values.
//
// Supported strace flags: -f (follow forks), -T (show time spent in syscall).
// Lines are expected in one of these forms:
//
//	syscall(args) = retval <latency>
//	[pid N] syscall(args) = retval <latency>
//	[pid N] syscall(args) = -1 ERRNAME (description) <latency>
package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

var (
	// pidRe matches the optional "[pid N] " prefix added by strace -f.
	pidRe = regexp.MustCompile(`^\[pid\s+(\d+)\]\s+`)

	// nameRe matches the syscall name at the start of (what remains of) a line.
	nameRe = regexp.MustCompile(`^(\w+)\(`)

	// retRe matches the tail of a completed syscall line:
	//   ) = retval [ERRNAME (description)] [<latency>]
	// Retval may be a decimal integer or a hex address (e.g. mmap returns 0x7f...).
	retRe = regexp.MustCompile(`\)\s+=\s+(-?\d+|0x[0-9a-fA-F]+)(?:\s+(E\w+)[^<]*)?(?:\s+<([\d.]+)>)?$`)

	// resumedRe matches a "<... syscall resumed>" line produced by strace -f.
	resumedRe = regexp.MustCompile(`^<\.\.\.\s+(\w+)\s+resumed>(.*)$`)
)

// Parse parses a single line of strace output.
// Returns nil, nil for non-syscall lines (signals, exit messages, unfinished stubs).
func Parse(line string, defaultPID int) (*models.SyscallEvent, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, nil
	}

	// Skip unfinished stubs; they will appear again as "resumed" lines.
	if strings.Contains(line, "<unfinished ...>") {
		return nil, nil
	}

	pid := defaultPID

	// Strip optional [pid N] prefix.
	if m := pidRe.FindStringSubmatch(line); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return nil, fmt.Errorf("invalid pid in %q: %w", m[0], err)
		}
		pid = n
		line = line[len(m[0]):]
	}

	// Handle "resumed" lines: <... syscall resumed> rest
	if strings.HasPrefix(line, "<...") {
		if m := resumedRe.FindStringSubmatch(line); m != nil {
			line = m[1] + "(" + m[2] // reconstruct enough to match retRe
		} else {
			return nil, nil
		}
	}

	// Require a syscall name.
	nm := nameRe.FindStringSubmatch(line)
	if nm == nil {
		return nil, nil
	}
	syscallName := nm[1]

	// Require a return value (filters out incomplete lines).
	rm := retRe.FindStringSubmatch(line)
	if rm == nil {
		return nil, nil
	}
	retVal := rm[1]
	errName := rm[2]
	latencyStr := rm[3]

	var latency time.Duration
	if latencyStr != "" {
		secs, err := strconv.ParseFloat(latencyStr, 64)
		if err == nil {
			latency = time.Duration(secs * float64(time.Second))
		}
	}

	// Extract args: text between the opening '(' and the last ') = '.
	argsStr := ""
	if eqIdx := strings.LastIndex(line, ") = "); eqIdx > len(syscallName) {
		argsStr = line[len(syscallName)+1 : eqIdx]
	}

	return &models.SyscallEvent{
		PID:     pid,
		Name:    syscallName,
		Args:    argsStr,
		RetVal:  retVal,
		Error:   errName,
		Latency: latency,
		Time:    time.Now(),
	}, nil
}
