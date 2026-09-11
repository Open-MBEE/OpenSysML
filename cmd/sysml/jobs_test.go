package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

// TestJSONReportsThePlanWorkers checks that -json carries under plan how many
// workers a plan built and how long their warming took, that an exploration under
// -jobs 1 and -jobs 8 reports the same document apart from those two figures,
// and that the human-readable report does not print them.
func TestJSONReportsThePlanWorkers(t *testing.T) {
	binary := buildCLI(t)

	type planReport struct {
		Checks []struct {
			Plan *struct {
				Workers int     `json:"workers"`
				Warming float64 `json:"warming"`
			} `json:"plan"`
		} `json:"checks"`
	}
	answer := func(jobs string) (string, planReport) {
		got := check(t, binary, forkModel, "-json", "-jobs", jobs, "-engine", "explore", "-action", "Mission::race")
		var report planReport
		if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
			t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
		}
		if len(report.Checks) != 1 || report.Checks[0].Plan == nil {
			t.Fatalf("report does not carry the plan:\n%s", got.stdout)
		}
		return got.stdout, report
	}
	one, oneReport := answer("1")
	eight, eightReport := answer("8")
	if p := oneReport.Checks[0].Plan; p.Workers != 1 || p.Warming < 0 {
		t.Errorf("-jobs 1 built %d workers warming %v ms, want one worker", p.Workers, p.Warming)
	}
	if p := eightReport.Checks[0].Plan; p.Workers < 1 || p.Workers > 8 || p.Warming < 0 {
		t.Errorf("-jobs 8 built %d workers warming %v ms, want between one and eight", p.Workers, p.Warming)
	}

	// The job count and the warming describe the run, not the answer: apart from
	// them the two documents are the same bytes.
	scrub := regexp.MustCompile(`"(workers|warming)": [0-9.]+`)
	if a, b := scrub.ReplaceAllString(one, `"$1": 0`), scrub.ReplaceAllString(eight, `"$1": 0`); a != b {
		t.Errorf("-jobs 1 reported\n%s\n-jobs 8 reported\n%s", one, eight)
	}

	got := check(t, binary, forkModel, "-jobs", "8", "-engine", "explore", "-action", "Mission::race")
	if got.status != 0 || strings.Contains(got.output(), "worker") || strings.Contains(got.output(), "warming") {
		t.Errorf("the human-readable report prints the workers:\n%s", got.output())
	}
}
