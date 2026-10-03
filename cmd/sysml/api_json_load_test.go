package main

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runValidatePath(t *testing.T, binary, path string) runOutcome {
	t.Helper()
	cmd := exec.Command(binary, "-validate", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result := runOutcome{stdout: stdout.String(), stderr: stderr.String()}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		result.status = exit.ExitCode()
	default:
		t.Fatalf("running -validate %s: %v\n%s", path, err, result.output())
	}
	return result
}

func apiJSONFixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "tests", "export", "testdata", "interchange", name))
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestValidateAPIJSONFiles(t *testing.T) {
	binary := buildCLI(t)

	flowEnds := runValidatePath(t, binary, apiJSONFixture(t, "flow_ends.toolkit.full.json"))
	if flowEnds.status != 0 || !strings.Contains(flowEnds.output(), "no errors") {
		t.Errorf("validating toolkit flow ends exited %d:\n%s", flowEnds.status, flowEnds.output())
	}

	library := runValidatePath(t, binary, apiJSONFixture(t, "library_identity.toolkit.full.json"))
	if library.status != 0 || !strings.Contains(library.output(), "warning: the library element") {
		t.Errorf("validating toolkit library identities exited %d:\n%s", library.status, library.output())
	}
}

func TestValidateUnconvertibleAPIJSONNamesFile(t *testing.T) {
	binary := buildCLI(t)
	path := apiJSONFixture(t, "library_unmatched.toolkit.full.json")

	result := runValidatePath(t, binary, path)
	if result.status == 0 || !strings.Contains(result.output(), path) {
		t.Fatalf("failed conversion did not name %s (exit %d):\n%s", path, result.status, result.output())
	}
}
