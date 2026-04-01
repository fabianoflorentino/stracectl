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

// buildFakeProcWithComm creates a fake /proc tree with cgroup, comm, and
// cmdline files so we can test the comm/cmdline fallback path.
func buildFakeProcWithComm(t *testing.T, procs map[int]struct{ cgroup, comm, cmdline string }) string {
	t.Helper()
	root := t.TempDir()
	for pid, p := range procs {
		dir := filepath.Join(root, strconv.Itoa(pid))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cgroup"), []byte(p.cgroup), 0o644); err != nil {
			t.Fatalf("write cgroup: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "comm"), []byte(p.comm), 0o644); err != nil {
			t.Fatalf("write comm: %v", err)
		}
		// cmdline uses NUL separators between argv components.
		if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(p.cmdline), 0o644); err != nil {
			t.Fatalf("write cmdline: %v", err)
		}
	}
	return root
}

// TestScanProcLowest_FallbackComm verifies that when the cgroup path contains
// only a hex container ID (as emitted by containerd/kind with cgroupv2), the
// fallback still resolves the PID by matching the cmdline fragment.
func TestScanProcLowest_FallbackComm(t *testing.T) {
	procs := map[int]struct{ cgroup, comm, cmdline string }{
		// hex cgroup IDs — container name not in path; simulates kind/containerd cgroupv2
		7:  {"0::/../cri-containerd-aabbccdd.scope\n", "sh\n", "/bin/sh\x00-c\x00while true; do sleep 5; done\x00"},
		18: {"0::/\n", "stracectl\n", "/usr/local/bin/stracectl\x00attach\x00"},
	}
	root := buildFakeProcWithComm(t, procs)

	pid, err := discover.ScanProcLowest(root, "sleep 5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != 7 {
		t.Fatalf("expected PID 7, got %d", pid)
	}
}

// TestScanProcLowest_FallbackCommName verifies matching via the short comm name.
func TestScanProcLowest_FallbackCommName(t *testing.T) {
	procs := map[int]struct{ cgroup, comm, cmdline string }{
		42: {"0::/../cri-containerd-deadbeef.scope\n", "nginx\n", "nginx\x00-g\x00daemon off;\x00"},
		99: {"0::/../cri-containerd-cafebabe.scope\n", "stracectl\n", "/usr/local/bin/stracectl\x00"},
	}
	root := buildFakeProcWithComm(t, procs)

	pid, err := discover.ScanProcLowest(root, "nginx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid != 42 {
		t.Fatalf("expected PID 42, got %d", pid)
	}
}
