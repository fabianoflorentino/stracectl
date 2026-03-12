package tracer

import (
	"reflect"
	"runtime"
	"testing"

	"golang.org/x/sys/unix"
)

// writeRelease writes a release string into a unix.Utsname.Release field
// using reflection so it works whether the element type is int8 or uint8.
func writeRelease(u *unix.Utsname, s string) {
	rv := reflect.ValueOf(&u.Release).Elem()
	// zero out
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		switch elem.Kind() {
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			elem.SetInt(0)
		default:
			elem.SetUint(0)
		}
	}
	for i := 0; i < rv.Len() && i < len(s); i++ {
		elem := rv.Index(i)
		b := s[i]
		switch elem.Kind() {
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
			elem.SetInt(int64(int8(b)))
		default:
			elem.SetUint(uint64(b))
		}
	}
}

func TestEbpfAvailable_KernelAndBuild(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires linux")
	}

	origUname := unameFunc
	origBuild := ebpfBuild
	t.Cleanup(func() {
		unameFunc = origUname
		ebpfBuild = origBuild
	})

	// Simulate kernel 5.8 and ebpf build enabled
	ebpfBuild = true
	unameFunc = func(u *unix.Utsname) error {
		writeRelease(u, "5.8.0-test")
		return nil
	}

	if !ebpfAvailable() {
		t.Fatal("expected ebpfAvailable() = true for kernel 5.8 with ebpfBuild=true")
	}

	// Simulate older kernel
	unameFunc = func(u *unix.Utsname) error {
		writeRelease(u, "5.7.9")
		return nil
	}
	if ebpfAvailable() {
		t.Fatal("expected ebpfAvailable() = false for kernel 5.7.9")
	}
}

func TestSelect_Auto_RespectsAvailability(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("requires linux")
	}

	origUname := unameFunc
	origBuild := ebpfBuild
	t.Cleanup(func() {
		unameFunc = origUname
		ebpfBuild = origBuild
	})

	// When build enables ebpf and kernel >= 5.8, Select("auto") should pick EBPF.
	ebpfBuild = true
	unameFunc = func(u *unix.Utsname) error {
		writeRelease(u, "5.8.1")
		return nil
	}
	tr, err := Select("auto")
	if err != nil {
		t.Fatalf("Select(auto) returned error: %v", err)
	}
	if _, ok := tr.(*EBPFTracer); !ok {
		t.Fatalf("Select(auto) returned %T, want *EBPFTracer", tr)
	}

	// When kernel < 5.8, Select("auto") should fall back to strace.
	unameFunc = func(u *unix.Utsname) error {
		writeRelease(u, "5.7.0")
		return nil
	}
	tr2, err := Select("auto")
	if err != nil {
		t.Fatalf("Select(auto) returned error: %v", err)
	}
	if _, ok := tr2.(*StraceTracer); !ok {
		t.Fatalf("Select(auto) returned %T for old kernel, want *StraceTracer", tr2)
	}
}
