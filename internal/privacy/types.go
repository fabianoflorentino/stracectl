package privacy

import (
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

// Arg represents a syscall argument or payload (trimmed according to config).
type Arg struct {
	Name  string
	Value []byte
	Meta  map[string]string
}

// TraceEvent is the internal pipeline event that flows through filters/redactor/formatter.
type TraceEvent struct {
	Ts         time.Time
	PID        int
	UID        int
	Comm       string
	Syscall    string
	Args       []Arg
	Ret        string
	Errno      string
	Metadata   map[string]string
	RawPayload []byte
}

// Convert from existing models.SyscallEvent for compatibility.
func NewTraceEventFromModel(m models.SyscallEvent) TraceEvent {
	return TraceEvent{
		Ts:       m.Time,
		PID:      m.PID,
		Comm:     "",
		Syscall:  m.Name,
		Ret:      m.RetVal,
		Errno:    m.Error,
		Metadata: map[string]string{},
	}
}

// Core pipeline interfaces.
type Filter interface {
	Allow(e *TraceEvent) bool
}

type Redactor interface {
	Redact(e *TraceEvent) error
}

type Formatter interface {
	Format(e *TraceEvent) ([]byte, error)
}

type Output interface {
	Write([]byte) error
	Close() error
}
