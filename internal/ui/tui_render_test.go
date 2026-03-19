package ui

import (
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/models"
)

func TestRenderLogAndFiles(t *testing.T) {
	a := aggregator.New()
	// add some entries
	a.Add(models.SyscallEvent{Name: "open", Args: "\"/tmp/foo\", O_RDONLY", RetVal: "3", PID: 1, Time: time.Now()})
	a.Add(models.SyscallEvent{Name: "read", Args: "3, 64", RetVal: "0", PID: 1, Time: time.Now()})

	m := model{agg: a, width: 80, height: 10, logOffset: -1, filesOffset: 0}
	logView := m.renderLog()
	if logView == "" {
		t.Fatal("expected non-empty log view")
	}
	if !contains(logView, "live log") {
		t.Fatalf("expected live log title in view: %q", logView)
	}

	filesView := m.renderFiles()
	if filesView == "" {
		t.Fatal("expected non-empty files view")
	}
	if !contains(filesView, "/tmp/foo") {
		t.Fatalf("expected /tmp/foo in files view: %q", filesView)
	}

	// render detail overlay for a syscall
	m2 := model{agg: a, width: 80, height: 24, cursor: 0}
	detail := m2.renderDetail()
	if detail == "" {
		t.Fatal("expected non-empty detail view")
	}
	if !contains(detail, "SYSCALL REFERENCE") {
		t.Fatalf("expected detail to contain reference section")
	}

	// exercise category bar
	m3 := model{agg: a, width: 80}
	cat := m3.renderCategoryBar(80)
	if cat == "" {
		t.Fatal("expected non-empty category bar")
	}
	if !contains(cat, "I/O") {
		t.Fatalf("expected I/O category in category bar, got %q", cat)
	}
}

// contains is a tiny helper less strict than strings.Contains to avoid
// importing extra packages in this test file.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (len(sub) == 0 || (len(s) > 0 && (indexOf(s, sub) >= 0)))
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
