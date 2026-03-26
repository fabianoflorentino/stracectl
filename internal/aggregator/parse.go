package aggregator

import (
	"strconv"
	"strings"
)

const (
	maxPathLen = 1024
)

// extractPathFromArgs attempts to heuristically extract the first path-like
// argument from a strace-style syscall args string. It prefers quoted strings
// and falls back to comma-splitting for open/openat.
func extractPathFromArgs(name, args string) string {
	pathSyscalls := map[string]bool{
		"open": true, "openat": true, "creat": true,
		"stat": true, "fstat": true, "lstat": true, "newfstatat": true, "statx": true,
		"access": true, "faccessat": true,
		"execve": true, "execveat": true,
		"readlink": true, "readlinkat": true,
		"symlink": true, "symlinkat": true,
		"unlink": true, "unlinkat": true,
		"rename": true, "renameat": true, "renameat2": true,
		"link": true, "linkat": true,
		"mkdir": true, "mkdirat": true, "rmdir": true,
		"chdir": true,
	}

	if !pathSyscalls[name] {
		return ""
	}

	// 1) look for a quoted string "..." and unescape it
	if i := strings.Index(args, "\""); i >= 0 {
		if j := strings.Index(args[i+1:], "\""); j >= 0 {
			s := args[i+1 : i+1+j]
			return unescapePath(s)
		}
	}

	// 2) fallback: split by commas and pick the likely argument for open/openat-like calls
	parts := strings.SplitN(args, ",", 3)
	var cand string
	switch name {
	case "open":
		if len(parts) >= 1 {
			cand = strings.TrimSpace(parts[0])
		}
	case "openat":
		if len(parts) >= 2 {
			cand = strings.TrimSpace(parts[1])
		}
	case "creat":
		if len(parts) >= 1 {
			cand = strings.TrimSpace(parts[0])
		}
	default:
		// For other path-taking syscalls, we don't attempt the numeric fallback.
	}
	// sanitize common non-path tokens (NULL or numeric/pointer-like values)
	if cand == "" || cand == "NULL" || cand == "0" || strings.HasPrefix(cand, "0x") {
		return ""
	}
	// If the candidate is quoted (e.g. '"/path"'), strip quotes and unescape.
	if strings.HasPrefix(cand, "\"") && strings.HasSuffix(cand, "\"") {
		return unescapePath(cand[1 : len(cand)-1])
	}
	return cand
}

// unescapePath attempts to handle C-style escapes using strconv.Unquote.
func unescapePath(s string) string {
	if unq, err := strconv.Unquote("\"" + s + "\""); err == nil {
		// Reject strings that contain control characters (including NUL)
		// as they are very likely to be binary payloads rather than paths.
		for _, r := range unq {
			if r == '\x00' || (r < 32 && r != '\t') {
				return ""
			}
		}
		return unq
	}
	// If Unquote failed, fall back to returning the raw input only if it
	// doesn't contain control characters.
	for _, r := range s {
		if r == '\x00' || (r < 32 && r != '\t') {
			return ""
		}
	}
	return s
}

// parseFirstIntArg attempts to parse the first comma-separated argument as an int.
// Returns (value, true) on success.
func parseFirstIntArg(args string) (int, bool) {
	parts := strings.SplitN(args, ",", 2)
	if len(parts) == 0 {
		return 0, false
	}
	s := strings.TrimSpace(parts[0])
	if s == "" {
		return 0, false
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v, true
	}
	return 0, false
}

// parseRetInt parses a syscall return value (decimal or hex) into int.
func parseRetInt(ret string) (int, bool) {
	if ret == "" {
		return 0, false
	}
	if v, err := strconv.Atoi(ret); err == nil {
		return v, true
	}
	if strings.HasPrefix(ret, "0x") {
		if v, err := strconv.ParseInt(ret, 0, 0); err == nil {
			return int(v), true
		}
	}
	return 0, false
}
