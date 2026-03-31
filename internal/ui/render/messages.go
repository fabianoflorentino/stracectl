package render

// Explanation messages for known syscalls. Kept in a separate file to
// make translations or text changes easier without touching logic.
const (
	explIoctl    = "terminal control failed — process likely has no TTY (running under sudo or piped)"
	explOpen     = "files not found — often normal (dynamic linker searches multiple paths)"
	explAccess   = "optional files are missing — usually harmless (checking for config files)"
	explConnect  = "connection attempts failed — may be Happy Eyeballs (IPv4/IPv6 race) or no route"
	explRecvFrom = "EAGAIN on non-blocking socket — normal for async I/O, not a real error"
	explSendTo   = "send failed — peer may have closed the connection"
	explMadvise  = "memory hint rejected by kernel — informational, not a real failure"
	explPrctl    = "process control rejected — may lack capabilities (seccomp or container policy)"
	explStatfs   = "filesystem stat failed — path may be on a special fs (proc, tmpfs)"
	explUnlink   = "tried to delete a non-existent file — may be cleanup of temp files"
	explMkdir    = "directory already exists — common during first-run initialisation"
)
