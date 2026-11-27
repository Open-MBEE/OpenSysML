package derived

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRunExporterRunsWithoutATimeout: -timeout 0 is no limit, not a deadline already past.
func TestRunExporterRunsWithoutATimeout(t *testing.T) {
	launcher := stubBridge(t, "", 0, stubFiles...)
	if _, err := runExporter(launcher, t.TempDir(), t.TempDir(), stubFiles, 0, newMetaclasses()); err != nil {
		t.Fatalf("got %v, want a clean run", err)
	}
}

func TestRunExporterStopsALauncherPastItsTimeout(t *testing.T) {
	launcher := filepath.Join(t.TempDir(), "export-json")
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\nexec sleep 600\n"), 0o700); err != nil { // #nosec G306 -- an executable test stub
		t.Fatal(err)
	}
	_, err := runExporter(launcher, t.TempDir(), t.TempDir(), stubFiles, 200*time.Millisecond, newMetaclasses())
	if err == nil || !strings.Contains(err.Error(), "exceeded 200ms") {
		t.Fatalf("got %v, want the timeout named", err)
	}
}
