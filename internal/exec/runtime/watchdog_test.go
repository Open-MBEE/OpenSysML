package runtime

import "time"

// watchdog returns a channel that fires after d scaled for slow runs: the step
// budget bounds every evaluation, so the wall clock only catches a true hang,
// and under -race on a loaded CI executor honest work runs 10x slower.
func watchdog(d time.Duration) <-chan time.Time {
	return time.After(d * watchdogScale)
}

const watchdogScale = 6
