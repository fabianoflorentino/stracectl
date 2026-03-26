package aggregator

import (
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

func TestEventHelpers_Basic(t *testing.T) {
	e := models.SyscallEvent{Name: "read", Latency: 10 * time.Microsecond}
	if e.Name != "read" {
		t.Fatalf("event helper created wrong name")
	}
}
