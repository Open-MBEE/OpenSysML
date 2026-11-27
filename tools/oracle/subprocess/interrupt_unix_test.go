//go:build unix

package subprocess

import (
	"errors"
	"os"
	"os/exec"
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

// ignoredHangup marks the re-run of this test binary that inherits an ignored hangup.
const ignoredHangup = "SUBPROCESS_TEST_IGNORED_HANGUP"

// TestAnIgnoredHangupStaysIgnored: a run started under nohup inherits the hangup as
// ignored, so a closing terminal must not end its helper.
func TestAnIgnoredHangupStaysIgnored(t *testing.T) {
	shell := sh(t)
	if os.Getenv(ignoredHangup) == "" {
		// #nosec G204 -- the test binary re-runs itself.
		child := exec.Command(shell, "-c", `trap "" HUP; exec "$0" -test.run='^TestAnIgnoredHangupStaysIgnored$'`, os.Args[0])
		child.Env = append(os.Environ(), ignoredHangup+"=1")
		if out, err := child.CombinedOutput(); err != nil {
			t.Fatalf("with the hangup ignored: %v\n%s", err, out)
		}
		return
	}
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
	}()
	if _, _, err := Output(2*time.Second, 1<<10, shell, "-c", "sleep 600 | cat"); !errors.Is(err, ErrTimeout) {
		t.Fatalf("got %v, want the hangup ignored and ErrTimeout", err)
	}
}
