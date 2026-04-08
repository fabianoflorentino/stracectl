package ui

import (
	"testing"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
)

func Test_LogFilesOffsetPtrs(t *testing.T) {
	agg := aggregator.New()
	m := ModelFromAggregator(agg, "proc", nil)
	pm := &m

	lp := pm.LogOffsetPtr()
	fp := pm.FilesOffsetPtr()

	*lp = 7
	*fp = 3

	if *pm.LogOffsetPtr() != 7 {
		t.Fatalf("expected log offset 7, got %d", *pm.LogOffsetPtr())
	}
	if *pm.FilesOffsetPtr() != 3 {
		t.Fatalf("expected files offset 3, got %d", *pm.FilesOffsetPtr())
	}
}
