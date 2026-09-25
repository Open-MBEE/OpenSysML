package analysis

import (
	goruntime "runtime"
	"testing"
)

// The default jobs are one per CPU while the memory available leaves each worker its
// budget, fewer as it runs short, one at least; unknown memory leaves one per CPU.
func TestDefaultJobsAreBoundedByTheMemoryAvailable(t *testing.T) {
	for _, tc := range []struct {
		available  int64
		cpus, want int
	}{
		{0, 8, 8},
		{-1, 8, 8},
		{64 * JobMemoryBudget, 8, 8},
		{8 * JobMemoryBudget, 8, 8},
		{8*JobMemoryBudget - 1, 8, 7},
		{3 * JobMemoryBudget, 8, 3},
		{JobMemoryBudget / 2, 8, 1},
		{1, 8, 1},
		{64 * JobMemoryBudget, 1, 1},
	} {
		if got := jobsForMemory(tc.available, tc.cpus); got != tc.want {
			t.Errorf("jobsForMemory(%d, %d) = %d, want %d", tc.available, tc.cpus, got, tc.want)
		}
	}
	if got := DefaultJobs(); got < 1 || got > goruntime.NumCPU() {
		t.Errorf("DefaultJobs() = %d, want within [1, %d]", got, goruntime.NumCPU())
	}
}

// The figures are read off the kernel's files as they are written: MemAvailable in kB,
// the cgroup's limit in bytes or "max", each 0 when absent or unreadable.
func TestAvailableMemoryReadsTheKernelsFigures(t *testing.T) {
	meminfo := []byte("MemTotal:       32000000 kB\nMemFree:         1000000 kB\nMemAvailable:    2000000 kB\n")
	if got := memAvailable(meminfo); got != 2000000<<10 {
		t.Errorf("MemAvailable = %d, want %d", got, 2000000<<10)
	}
	for _, bad := range []string{"", "MemFree: 5 kB\n", "MemAvailable: lots kB\n"} {
		if got := memAvailable([]byte(bad)); got != 0 {
			t.Errorf("memAvailable(%q) = %d, want 0", bad, got)
		}
	}
	for _, tc := range []struct {
		limit, current string
		want           int64
	}{
		{"max\n", "100\n", 0},
		{"", "100", 0},
		{"lots", "100", 0},
		{"4000\n", "1000\n", 3000},
		{"4000", "", 4000},
		{"4000", "5000", 1},
	} {
		if got := cgroupHeadroom([]byte(tc.limit), []byte(tc.current)); got != tc.want {
			t.Errorf("cgroupHeadroom(%q, %q) = %d, want %d", tc.limit, tc.current, got, tc.want)
		}
	}
}
