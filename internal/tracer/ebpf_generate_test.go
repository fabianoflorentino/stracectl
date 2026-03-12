package tracer

import "testing"

// TestLoadEbpfSpec verifies that the embedded eBPF ELF blob parses into a
// CollectionSpec. This ensures the generated artifacts are present and valid.
func TestLoadEbpfSpec(t *testing.T) {
	spec, err := loadEbpf()
	if err != nil {
		t.Fatalf("loadEbpf returned error: %v", err)
	}
	if spec == nil {
		t.Fatalf("loadEbpf returned nil spec")
	}
}
