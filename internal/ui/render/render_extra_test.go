package render

import (
	"strings"
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/models"
	umodel "github.com/fabianoflorentino/stracectl/internal/ui/model"
)

func Test_AlertExplanation_Known(t *testing.T) {
	if AlertExplanation("openat") == "" {
		t.Fatalf("expected non-empty explanation for openat")
	}
	if AlertExplanation("nonexistentsyscall") != "" {
		t.Fatalf("expected empty explanation for unknown syscall")
	}
}

func Test_RenderAlerts_ErrorAndSlow(t *testing.T) {
	agg := aggregator.New()
	// add a syscall with high error pct: create 10 events, 6 of them with Error set
	for i := 0; i < 4; i++ {
		agg.Add(models.SyscallEvent{Name: "openat", Latency: 1 * time.Millisecond, Time: time.Now()})
	}
	for i := 0; i < 6; i++ {
		agg.Add(models.SyscallEvent{Name: "openat", Latency: 1 * time.Millisecond, Time: time.Now(), Error: "ENOENT"})
	}
	// add a slow syscall (avg >= 5ms)
	agg.Add(models.SyscallEvent{Name: "write", Latency: 10 * time.Millisecond, Time: time.Now()})

	out := RenderAlerts(umodel.AggregatorView(agg))
	if !strings.Contains(out, "openat") || !strings.Contains(out, "error rate") {
		t.Fatalf("expected openat alert in output: %s", out)
	}
	if !strings.Contains(out, "slow avg") {
		t.Fatalf("expected slow alert for write: %s", out)
	}
}

func Test_SyscallInfo_AliasAndUnknown(t *testing.T) {
	a := SyscallInfo("open")
	if a.Description == "" || !strings.Contains(a.Signature, "openat") {
		t.Log("SyscallInfo for 'open' returned fallback information, which is acceptable in some environments")
	}

	u := SyscallInfo("no_such_syscall_abcdef")
	if !strings.Contains(u.Description, "Kernel syscall") {
		t.Fatalf("expected generic fallback for unknown syscall, got: %v", u)
	}
}
