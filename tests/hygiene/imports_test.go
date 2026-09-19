// Package hygiene holds module-wide checks that no single package owns.
package hygiene

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoProductionCodeImportsTesting keeps the test framework out of the shipped
// binaries: a file named so Go compiles it into the package proper (say
// `x_test2.go`) links `testing` and never runs as a test.
func TestNoProductionCodeImportsTesting(t *testing.T) {
	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}} {{join .Imports \" \"}}", "./...")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		for _, imp := range fields[1:] {
			if imp == "testing" {
				t.Errorf("%s imports testing from non-test files", fields[0])
			}
		}
	}
}
