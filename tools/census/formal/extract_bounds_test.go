package formal

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tools/oracle/subprocess"
)

// TestTextOfStopsAConverterThatFloodsItsOutput: a converter that never stops writing is
// stopped at the bound rather than held in memory to the end.
func TestTextOfStopsAConverterThatFloodsItsOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in converter is a POSIX shell script")
	}
	converter := filepath.Join(t.TempDir(), "pdftotext")
	if err := os.WriteFile(converter, []byte("#!/bin/sh\nexec yes 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef\n"), 0o700); err != nil { // #nosec G306 -- an executable test stub
		t.Fatal(err)
	}
	if text, err := textOf(converter, "spec.pdf"); !errors.Is(err, subprocess.ErrOutput) {
		t.Fatalf("got %d bytes, %v; want the output bound", len(text), err)
	}
}
