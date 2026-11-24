//go:build !windows

package subprocess

import (
	"errors"
	"syscall"
	"testing"
	"time"
)

// TestAnInterruptStopsTheHelper: the helper is outside the terminal's foreground group,
// so an interrupt of this process has to end it.
func TestAnInterruptStopsTheHelper(t *testing.T) {
	shell := sh(t)
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()
	start := time.Now()
	_, _, err := Output(time.Hour, 1<<10, shell, "-c", "sleep 600 | cat")
	if !errors.Is(err, ErrInterrupted) {
		t.Fatalf("got %v, want ErrInterrupted", err)
	}
	if elapsed := time.Since(start); elapsed > WaitDelay/2 {
		t.Fatalf("returned after %s, so the helper outlived the interrupt", elapsed)
	}
}
