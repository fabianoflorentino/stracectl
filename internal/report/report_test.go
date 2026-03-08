package report_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fabianoflorentino/stracectl/internal/aggregator"
	"github.com/fabianoflorentino/stracectl/internal/models"
	"github.com/fabianoflorentino/stracectl/internal/report"
)

func TestWrite(t *testing.T) {
	agg := aggregator.New()
	events := []models.SyscallEvent{
		{Name: "openat", Latency: 45 * time.Microsecond, Time: time.Now()},
		{Name: "read", Latency: 12 * time.Microsecond, Time: time.Now()},
		{Name: "openat", Latency: 50 * time.Microsecond, Time: time.Now()},
		{Name: "write", Latency: 3 * time.Microsecond, Time: time.Now()},
		{Name: "openat", Error: "ENOENT", Latency: 6 * time.Microsecond, Time: time.Now()},
		{Name: "execve", Latency: 124 * time.Microsecond, Time: time.Now()},
	}
	for _, e := range events {
		agg.Add(e)
	}

	f, err := os.CreateTemp("", "stracectl-report-*.html")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	f.Close()
	defer os.Remove(path)

	if err := report.Write(path, agg, "test run"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	html := string(content)

	for _, want := range []string{
		"<!DOCTYPE html>",
		"stracectl report",
		"openat",
		"execve",
		"test run",
		"Total calls",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("report missing %q", want)
		}
	}
}
