package formatter

import (
	"encoding/json"

	"github.com/fabianoflorentino/stracectl/internal/privacy"
)

// JSONFormatter writes TraceEvent as JSON. It assumes redaction already applied.
type JSONFormatter struct{}

// NewJSONFormatter creates a JSONFormatter.
func NewJSONFormatter() *JSONFormatter { return &JSONFormatter{} }

type jsonArg struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type jsonEvent struct {
	Ts       string            `json:"ts"`
	PID      int               `json:"pid"`
	Comm     string            `json:"comm,omitempty"`
	Syscall  string            `json:"syscall"`
	Args     []jsonArg         `json:"args,omitempty"`
	Ret      string            `json:"ret"`
	Errno    string            `json:"errno,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Format converts TraceEvent to JSON bytes.
func (f *JSONFormatter) Format(e *privacy.TraceEvent) ([]byte, error) {
	je := jsonEvent{
		Ts:       e.Ts.Format("2006-01-02T15:04:05.000Z07:00"),
		PID:      e.PID,
		Comm:     e.Comm,
		Syscall:  e.Syscall,
		Ret:      e.Ret,
		Errno:    e.Errno,
		Metadata: e.Metadata,
	}

	if len(e.Args) > 0 {
		args := make([]jsonArg, 0, len(e.Args))
		for _, a := range e.Args {
			args = append(args, jsonArg{Name: a.Name, Value: string(a.Value)})
		}
		je.Args = args
	}

	return json.Marshal(je)
}
