package cmd

import "testing"

func TestRunFlagsRegistered(t *testing.T) {
	if runCmd.Flags().Lookup("force-ebpf") == nil {
		t.Fatal("run: force-ebpf flag not registered")
	}
	if runCmd.Flags().Lookup("unfiltered") == nil {
		t.Fatal("run: unfiltered flag not registered")
	}
}
