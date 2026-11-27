package apijson

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLaunchPilotRunsWithoutATimeout: -timeout 0 is no limit, not a deadline already past.
func TestLaunchPilotRunsWithoutATimeout(t *testing.T) {
	run, err := launchPilot(stubBridge(t, "", 0, stubFiles...), t.TempDir(), t.TempDir(), nil, stubFiles, 0)
	if err != nil || run.Status != 0 {
		t.Fatalf("got status %d, %v; want a clean run", run.Status, err)
	}
}

func TestLaunchPilotStopsALauncherPastItsTimeout(t *testing.T) {
	launcher := filepath.Join(t.TempDir(), "export-json")
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\nexec sleep 600\n"), 0o700); err != nil { // #nosec G306 -- an executable test stub
		t.Fatal(err)
	}
	_, err := launchPilot(launcher, t.TempDir(), t.TempDir(), nil, stubFiles, 200*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "exceeded 200ms") {
		t.Fatalf("got %v, want the timeout named", err)
	}
}
