package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// checkEnv is check with variables added to the binary's environment.
func checkEnv(t *testing.T, binary, model string, env []string, args ...string) runOutcome {
	t.Helper()
	path := filepath.Join(t.TempDir(), "model.sysml")
	if err := os.WriteFile(path, []byte(model), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary, append(args, path)...)
	cmd.Env = append(os.Environ(), env...)
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
		t.Fatalf("%v: %v\n%s", args, err, result.output())
	}
	return result
}

// TestJobsFlagAndEnvironment checks that -jobs and OPENSYSML_JOBS take a positive
// integer, that the flag wins over the variable, that either rejected at startup
// runs nothing, and that the count does not change what a run reports.
func TestJobsFlagAndEnvironment(t *testing.T) {
	binary := buildCLI(t)
	want := check(t, binary, forkModel, "-schedule", "explore", "-action", "Mission::race")
	wantReport(t, want, 0, "outcomes")

	for _, jobs := range []string{"1", "2", "8"} {
		got := check(t, binary, forkModel, "-jobs", jobs, "-schedule", "explore", "-action", "Mission::race")
		if got.status != 0 || got.output() != want.output() {
			t.Errorf("-jobs %s reported\n%s\nwant the default's\n%s", jobs, got.output(), want.output())
		}
		got = checkEnv(t, binary, forkModel, []string{"OPENSYSML_JOBS=" + jobs}, "-schedule", "explore", "-action", "Mission::race")
		if got.status != 0 || got.output() != want.output() {
			t.Errorf("OPENSYSML_JOBS=%s reported\n%s\nwant the default's\n%s", jobs, got.output(), want.output())
		}
	}

	for _, bad := range []string{"0", "-3", "two", "1.5"} {
		got := check(t, binary, forkModel, "-jobs", bad, "-action", "Mission::race")
		if got.status != 2 || !strings.Contains(got.output(), `-jobs="`+bad+`" is not a positive integer`) || strings.Contains(got.output(), "x = ") {
			t.Errorf("-jobs %s: status %d\n%s", bad, got.status, got.output())
		}
		got = checkEnv(t, binary, forkModel, []string{"OPENSYSML_JOBS=" + bad}, "-action", "Mission::race")
		if got.status != 2 || !strings.Contains(got.output(), `OPENSYSML_JOBS="`+bad+`" is not a positive integer`) || strings.Contains(got.output(), "x = ") {
			t.Errorf("OPENSYSML_JOBS=%s: status %d\n%s", bad, got.status, got.output())
		}
	}

	got := checkEnv(t, binary, forkModel, []string{"OPENSYSML_JOBS=nope"}, "-jobs", "2", "-action", "Mission::race")
	wantReport(t, got, 0, "x = 1")
}
