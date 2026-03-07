package models_test

import (
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

func TestSyscallEvent_IsError(t *testing.T) {
	cases := []struct {
		name    string
		event   models.SyscallEvent
		wantErr bool
	}{
		{"no error", models.SyscallEvent{Name: "read", RetVal: "3"}, false},
		{"errno string", models.SyscallEvent{Name: "open", Error: "ENOENT"}, true},
		{"retval -1 no errno", models.SyscallEvent{Name: "stat", RetVal: "-1"}, false},
		{"empty event", models.SyscallEvent{}, false},
		{"with latency no error", models.SyscallEvent{
			Name:    "write",
			RetVal:  "10",
			Latency: 50 * time.Microsecond,
		}, false},
		{"with latency and error", models.SyscallEvent{
			Name:    "connect",
			RetVal:  "-1",
			Error:   "ECONNREFUSED",
			Latency: 100 * time.Microsecond,
		}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.event.IsError()
			if got != tc.wantErr {
				t.Errorf("IsError() = %v, want %v", got, tc.wantErr)
			}
		})
	}
}
