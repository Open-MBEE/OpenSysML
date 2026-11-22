package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHumanBytes(t *testing.T) {
	for _, tc := range []struct {
		n    uint64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{2048, "2.0 KiB"},
		{5 << 20, "5.0 MiB"},
		{3 << 30, "3.0 GiB"},
	} {
		if got := humanBytes(tc.n); got != tc.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

func TestReportMemStatsReportsWhatTheRunCost(t *testing.T) {
	var out strings.Builder
	reportMemStats(&out, 1500*time.Millisecond)
	got := out.String()
	for _, want := range []string{"1.5s wall", "allocated", "taken from the OS"} {
		if !strings.Contains(got, want) {
			t.Errorf("memstats line %q does not report %q", got, want)
		}
	}
}

// A run that asks for no profile pays for none, and the function that ends the
// profiles is safe to call regardless.
func TestStartProfilingWithoutFlags(t *testing.T) {
	defer resetProfileFlags()
	stop, err := startProfiling()
	if err != nil {
		t.Fatalf("startProfiling: %v", err)
	}
	stop()
}

func TestStartProfilingWritesProfiles(t *testing.T) {
	needs(t, profileFeature)
	defer resetProfileFlags()
	dir := t.TempDir()
	cpuProfilePath = filepath.Join(dir, "cpu.out")
	memProfilePath = filepath.Join(dir, "heap.out")

	stop, err := startProfiling()
	if err != nil {
		t.Fatalf("startProfiling: %v", err)
	}
	stop()

	for _, path := range []string{cpuProfilePath, memProfilePath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("profile %s: %v", filepath.Base(path), err)
		}
		if info.Size() == 0 {
			t.Errorf("profile %s is empty", filepath.Base(path))
		}
	}
}

// A profile that cannot be created is reported rather than silently skipped, and
// leaves no profile running to trip up the next run.
func TestStartProfilingReportsUnwritableFile(t *testing.T) {
	needs(t, profileFeature)
	defer resetProfileFlags()
	unwritable := filepath.Join(t.TempDir(), "missing", "cpu.out")

	cpuProfilePath = unwritable
	stop, err := startProfiling()
	if err == nil {
		stop()
		t.Fatal("startProfiling accepted a CPU profile path it cannot create")
	}
	stop()

	// The heap profile is opened after the CPU profile is started, so a failure
	// there must stop the CPU profile it already started.
	cpuProfilePath = filepath.Join(t.TempDir(), "cpu.out")
	memProfilePath = unwritable
	stop, err = startProfiling()
	if err == nil {
		stop()
		t.Fatal("startProfiling accepted a heap profile path it cannot create")
	}
	stop()

	// Proof that the failed run left no CPU profile running: starting one now
	// succeeds, which it would not if the previous one were still going.
	cpuProfilePath = filepath.Join(t.TempDir(), "again.out")
	memProfilePath = ""
	stop, err = startProfiling()
	if err != nil {
		t.Fatalf("startProfiling after a failed run: %v", err)
	}
	stop()
}

func resetProfileFlags() {
	cpuProfilePath = ""
	memProfilePath = ""
	memStats = false
}
