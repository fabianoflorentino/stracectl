package aggregator

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/models"
)

func TestCategoryJSON(t *testing.T) {
	b, err := json.Marshal(CatIO)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var c Category
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c != CatIO {
		t.Fatalf("roundtrip failed: got %v", c)
	}
}

func TestParseRetInt(t *testing.T) {
	if v, ok := parseRetInt("123"); !ok || v != 123 {
		t.Fatalf("parseRetInt decimal failed")
	}
	if v, ok := parseRetInt("0x1f"); !ok || v != 31 {
		t.Fatalf("parseRetInt hex failed: %d", v)
	}
	if _, ok := parseRetInt(""); ok {
		t.Fatalf("parseRetInt empty should fail")
	}
}

func TestParseFirstIntArg(t *testing.T) {
	if v, ok := parseFirstIntArg("4, foo"); !ok || v != 4 {
		t.Fatalf("parseFirstIntArg failed")
	}
	if _, ok := parseFirstIntArg(""); ok {
		t.Fatalf("parseFirstIntArg empty should fail")
	}
}

func TestUnescapeAndExtractPath(t *testing.T) {
	p := extractPathFromArgs("open", "\"/tmp/foo bar\", O_RDONLY")
	if p != "/tmp/foo bar" {
		t.Fatalf("expected /tmp/foo bar, got %q", p)
	}
	p2 := extractPathFromArgs("openat", "AT_FDCWD, \"/etc/hosts\", 0")
	if p2 != "/etc/hosts" {
		t.Fatalf("expected /etc/hosts, got %q", p2)
	}
	if extractPathFromArgs("read", "\"/tmp/x\"") != "" {
		t.Fatalf("expected empty for read")
	}
}

func TestTopFilesAndAttribution(t *testing.T) {
	a := New()
	a.Add(models.SyscallEvent{Name: "open", Args: "\"/tmp/foo\", O_RDONLY", RetVal: "3", PID: 1, Time: time.Now()})
	a.Add(models.SyscallEvent{Name: "close", Args: "3", RetVal: "", PID: 1, Time: time.Now()})
	files := a.TopFilesForSyscall("open", 0)
	if len(files) == 0 || files[0].Path != "/tmp/foo" {
		t.Fatalf("TopFilesForSyscall did not contain expected /tmp/foo; got %v", files)
	}
	top := a.TopFiles(0)
	found := false
	for _, f := range top {
		if f.Path == "/tmp/foo" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("TopFiles missing /tmp/foo")
	}
}
