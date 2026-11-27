package exporter

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/subprocess"
)

// TestJVMStopsALauncherThatFloodsItsAnswer: the answer is one line, so a launcher that
// never stops writing is stopped at the bound rather than read to the end.
func TestJVMStopsALauncherThatFloodsItsAnswer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the launcher is a POSIX shell script")
	}
	launcher := filepath.Join(t.TempDir(), "export-json")
	if err := os.WriteFile(launcher, []byte("#!/bin/sh\nwhile :; do echo 21; done\n"), 0o700); err != nil { // #nosec G306 -- an executable test stub
		t.Fatal(err)
	}
	if jvm, err := JVM(launcher); !errors.Is(err, subprocess.ErrOutput) {
		t.Fatalf("got %q, %v; want the output bound", jvm, err)
	}
}
