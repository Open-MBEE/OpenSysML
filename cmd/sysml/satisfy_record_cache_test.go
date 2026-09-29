package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestSatisfyOverRecordedFiles checks repeated CLI runs find assertions from the record cache.
func TestSatisfyOverRecordedFiles(t *testing.T) {
	binary := buildCLI(t)
	cacheHome := t.TempDir()
	model := filepath.Join("..", "..", "examples", "runtime-showcase", "delta-v-budget.sysml")

	for run := 1; run <= 3; run++ {
		cmd := exec.Command(binary, "-quiet", "-satisfy", model)
		cmd.Env = append(os.Environ(),
			"XDG_CACHE_HOME="+cacheHome,
			"OPENSYSML_RECORD_CACHE=1",
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("run %d: sysml exited with %v\n%s", run, err, output)
		}
		if !strings.Contains(string(output), "✓ satisfy saturnIBOrbit by saturnIB holds") {
			t.Errorf("run %d did not report the satisfaction verdict:\n%s", run, output)
		}
	}
}
