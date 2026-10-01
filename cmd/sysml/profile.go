package main

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"time"
)

// Profiling flags. A profile covers a whole run, so it starts once the command
// line is understood and is written when the run ends.
var (
	cpuProfilePath string
	memProfilePath string
	memStats       bool
)

// startProfiling begins the profiles the command line asked for and returns the
// function that ends them, writing what they recorded. Asking for none is not an
// error: the returned function is then a no-op.
func startProfiling() (func(), error) {
	started := time.Now()
	var ends []func()

	// The stop returned when no profile was started.
	noop := func() { /* nothing was started, so nothing to write */ }

	// The ends run in reverse of the order they were added, so each profile is
	// written before the one started before it is stopped.
	stop := func() {
		for i := len(ends) - 1; i >= 0; i-- {
			ends[i]()
		}
	}

	pprofEnds, err := startPprof()
	if err != nil {
		return noop, err
	}
	ends = append(ends, pprofEnds...)

	if memStats {
		ends = append(ends, func() { reportMemStats(os.Stderr, time.Since(started)) })
	}

	return stop, nil
}

// reportMemStats reports the time the run took, what it allocated in total (the
// pressure it put on the collector) and what it took from the OS (a floor on its
// peak resident size). Neither is the live size of the model, which is
// unreachable by the time a run ends; the benchmarks measure that.
func reportMemStats(w io.Writer, elapsed time.Duration) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Fprintf(w, "sysml: %s wall, %s allocated in %d allocations over %d collections, %s taken from the OS\n",
		elapsed.Round(time.Millisecond), humanBytes(m.TotalAlloc), m.Mallocs,
		m.NumGC, humanBytes(m.Sys))
}

// humanBytes writes a byte count in the largest unit that leaves a whole part.
func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f PiB", value)
}
