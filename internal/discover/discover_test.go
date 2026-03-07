package discover_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/fabianoflorentino/stracectl/internal/discover"
)

// buildFakeProc creates a temporary /proc-like directory with fake cgroup files.
func buildFakeProc(t *testing.T, procs map[int]string) string {
	t.Helper()
	root := t.TempDir()
	for pid, cgroup := range procs {
		dir := filepath.Join(root, strconv.Itoa(pid))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cgroup"), []byte(cgroup), 0o644); err != nil {
			t.Fatalf("write cgroup: %v", err)
		}
	}
	return root
}

func TestScanProc_Found(t *testing.T) {
	procs := map[int]string{
		100: "0::/kubepods/burstable/poda1b2/myapp-abc123\n",
		200: "0::/kubepods/burstable/poda1b2/sidecar-xyz\n",
	}
	root := buildFakeProc(t, procs)

	pid, err := discover.ScanProc(root, "myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != 100 {
		t.Fatalf("expected PID 100, got %d", pid)
	}
}

func TestScanProc_NotFound(t *testing.T) {
	procs := map[int]string{
		100: "0::/kubepods/burstable/poda1b2/myapp-abc123\n",
	}
	root := buildFakeProc(t, procs)

	_, err := discover.ScanProc(root, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent container")
	}
}

func TestScanProcLowest_ReturnsLowest(t *testing.T) {
	procs := map[int]string{
		300: "0::/kubepods/burstable/poda1b2/backend-xyz\n",
		150: "0::/kubepods/burstable/poda1b2/backend-xyz\n",
		500: "0::/kubepods/burstable/poda1b2/backend-xyz\n",
	}
	root := buildFakeProc(t, procs)

	pid, err := discover.ScanProcLowest(root, "backend")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != 150 {
		t.Fatalf("expected 150, got %d", pid)
	}
}

func TestScanProcLowest_NotFound(t *testing.T) {
	root := buildFakeProc(t, map[int]string{
		100: "0::/kubepods/other/container\n",
	})
	_, err := discover.ScanProcLowest(root, "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestScanProc_InvalidProcRoot(t *testing.T) {
	_, err := discover.ScanProc("/nonexistent/proc/path", "anything")
	if err == nil {
		t.Fatal("expected error for missing procRoot")
	}
}

func TestScanProcLowest_InvalidProcRoot(t *testing.T) {
	_, err := discover.ScanProcLowest("/nonexistent/proc/path", "anything")
	if err == nil {
		t.Fatal("expected error for missing procRoot")
	}
}

func TestContainerPID_LiveProc_Nonexistent(t *testing.T) {
	_, err := discover.ContainerPID("______nonexistent_container______")
	if err == nil {
		t.Fatal("expected error for nonexistent container")
	}
}

func TestLowestPIDInContainer_LiveProc_Nonexistent(t *testing.T) {
	_, err := discover.LowestPIDInContainer("______nonexistent_container______")
	if err == nil {
		t.Fatal("expected error for nonexistent container")
	}
}
