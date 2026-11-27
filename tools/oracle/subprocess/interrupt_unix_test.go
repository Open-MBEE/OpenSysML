//go:build unix

package subprocess

import (
	"errors"
	"syscall"
	"testing"
	"time"
)

// TestAnInterruptStopsTheHelper: the helper is outside the terminal's foreground group,
// so each signal that stops this process has to end it.
func TestAnInterruptStopsTheHelper(t *testing.T) {
	shell := sh(t)
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT} {
		t.Run(sig.String(), func(t *testing.T) {
			go func() {
				time.Sleep(300 * time.Millisecond)
				_ = syscall.Kill(syscall.Getpid(), sig)
			}()
			start := time.Now()
			_, _, err := Output(time.Hour, 1<<10, shell, "-c", "sleep 600 | cat")
			if !errors.Is(err, ErrInterrupted) {
				t.Fatalf("got %v, want ErrInterrupted", err)
			}
			if elapsed := time.Since(start); elapsed > WaitDelay/2 {
				t.Fatalf("returned after %s, so the helper outlived the signal", elapsed)
			}
		})
	}
}
