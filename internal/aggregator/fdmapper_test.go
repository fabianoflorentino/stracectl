package aggregator

import "testing"

func TestFDMapper_SetGetDelete(t *testing.T) {
	d := NewDefaultFDMapper()
	d.Set(1, 4, "/tmp/testfile")

	if p, ok := d.Get(1, 4); !ok || p != "/tmp/testfile" {
		t.Fatalf("Get after Set: want /tmp/testfile, got %v, %v", p, ok)
	}

	d.Delete(1, 4)
	if _, ok := d.Get(1, 4); ok {
		t.Fatalf("Get after Delete: expected missing, got present")
	}
}
