package aggregator

import "testing"

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
