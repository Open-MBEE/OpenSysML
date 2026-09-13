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

// The determinism fixtures beside the runtime's exploration tests.
const (
	laterPrefixViolatesFaster = "../../internal/core/runtime/testdata/later_prefix_violates_faster.sysml"
	slowFirstWriter           = "../../internal/core/runtime/testdata/conformance/action_explore_slow_first_writer.sysml"
)

// runFigures matches what a -json document says of the run and not of the answer: the
// workers a plan built and their warming, the time a sweep row took and the rule under a
// sweep table's header, whose width follows those times.
var runFigures = regexp.MustCompile(`"(workers|warming|milliseconds)": [0-9.]+|[0-9]+\.[0-9]{3}ms|-{2,}`)

// TestJSONIsTheSameOnOneJobAsOnEight checks that -jobs 1 and -jobs 8 report the same
// -json document over the determinism fixtures — the witness, the outcome table and the
// cut of a violation a later, wider prefix reaches faster; the same with runs set just
// above the witness; a slow first prefix beside wide siblings — and over a sweep, apart
// from the figures that describe the run.
func TestJSONIsTheSameOnOneJobAsOnEight(t *testing.T) {
	binary := buildCLI(t)
	read := func(path string) string {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	cases := []struct {
		name  string
		model string
		args  []string
		want  []string
	}{
		{"later prefix violates faster", read(laterPrefixViolatesFaster),
			[]string{"-schedule", "explore", "-action", "test::race"},
			[]string{`"error": "eval assignment RHS: division by zero"`, `"linearizations": 3`,
				`"step 2: decision pick -\u003e 1-\u003eslow"`, `"runs": 7`, `"complete": true`}},
		{"runs just above the witness", read(laterPrefixViolatesFaster),
			[]string{"-schedule", "explore:runs=2", "-action", "test::race"},
			[]string{`"error": "eval assignment RHS: division by zero"`, `"linearizations": 2`,
				`"step 2: decision pick -\u003e 1-\u003eslow"`, `"runs": 2`, `"complete": false`, `"runs"`}},
		{"slow first prefix beside wide siblings", read(slowFirstWriter),
			[]string{"-schedule", "explore", "-action", "test::race"},
			[]string{`"x = 144 | 2 `, `"x = 2   | 2 `, `"x = 3   | 2 `, `"runs": 6`, `"complete": true`}},
		{"sweep", sweepCLIModel,
			[]string{"-calc", "Sw::Ratio(b = 1.0)", "-sweep", "a=1..4"},
			[]string{`"value": "4.0"`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			answer := func(jobs string) string {
				got := check(t, binary, tc.model, append([]string{"-json", "-jobs", jobs}, tc.args...)...)
				if got.stderr != "" || !json.Valid([]byte(got.stdout)) {
					t.Fatalf("-jobs %s did not report a JSON document alone:\n%s", jobs, got.output())
				}
				return got.stdout
			}
			one, eight := answer("1"), answer("8")
			for _, want := range tc.want {
				if !strings.Contains(one, want) {
					t.Errorf("-jobs 1 does not report %s:\n%s", want, one)
				}
			}
			if a, b := runFigures.ReplaceAllString(one, ""), runFigures.ReplaceAllString(eight, ""); a != b {
				t.Errorf("-jobs 1 reported\n%s\n-jobs 8 reported\n%s", one, eight)
			}
		})
	}
}

// cellPadding matches the padding before a column rule of a table, which in the time
// column and its header follows the width of the longest time run.
var cellPadding = regexp.MustCompile(` +\|`)

// TestSweepReportsAreTheSameOnOneJobAsOnEight checks that every sweep the CLI tests run —
// the calc sweeps and their product, a sample, a case on the object -instantiate made, a
// verification on its subject, a trade study with a failing row — reports the same
// human-readable table and the same -json document under -jobs 1 as under -jobs 8,
// apart from the figures that describe the run: rows in plan order, the same outputs,
// verdicts, evaluations, subjects and failed-row errors, and the same exit status.
func TestSweepReportsAreTheSameOnOneJobAsOnEight(t *testing.T) {
	binary := buildCLI(t)
	cases := []struct {
		name  string
		model string
		args  []string
		want  string
	}{
		{"calc", sweepCLIModel, []string{"-calc", "Sw::Twice", "-sweep", "n=1..4"}, "4 | 8 "},
		{"failing row", sweepCLIModel, []string{"-calc", "Sw::Ratio(a = 4.0)", "-sweep", "b=-1..1"}, "division by zero"},
		{"product", sweepCLIModel, []string{"-calc", "Sw::Ratio", "-sweep", "a=1.0..2.0:1.0", "-sweep", "b=2.0..4.0:2.0"}, "2.0 | 4.0 | 0.5 "},
		{"samples", sweepCLIModel, []string{"-calc", "Sw::Twice", "-samples", "5", "-seed", "42", "-sweep", "n=1..100"}, "5 run(s), seed 42"},
		{"case on an object", sweepCLIModel, []string{"-instantiate", "Sw::ship", "-analysis", "Sw::Priced Sw::ship", "-sweep", "tax=0.0..1.0:0.5"}, "1.0 | 10.0 | obj: not satisfied "},
		{"verification", sweepVerificationModel, []string{"-analysis", "Sw::checkScout", "-sweep", "limit=1.0..2.0:0.5"}, "1.0 | VerdictKind::fail | obj: not satisfied "},
		{"trade study", tradeStudyModel, []string{"-analysis", "Trade::perOffset", "-sweep", "offset=3..4"}, "evaluationFunction(Trade::a (object #1)) = 10.0 [selected]; evaluationFunction(Trade::b (object #2)) = 10.0 [tied]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := func(jobs string, json bool) (string, int) {
				args := []string{"-jobs", jobs}
				if json {
					args = append(args, "-json")
				}
				got := check(t, binary, tc.model, append(args, tc.args...)...)
				out := runFigures.ReplaceAllString(got.output(), "")
				return cellPadding.ReplaceAllString(out, " |"), got.status
			}
			one, oneStatus := report("1", false)
			eight, eightStatus := report("8", false)
			if !strings.Contains(one, tc.want) {
				t.Fatalf("-jobs 1 does not report %s:\n%s", tc.want, one)
			}
			if one != eight || oneStatus != eightStatus {
				t.Errorf("-jobs 1 reported %d\n%s\n-jobs 8 reported %d\n%s", oneStatus, one, eightStatus, eight)
			}
			one, oneStatus = report("1", true)
			eight, eightStatus = report("8", true)
			if one != eight || oneStatus != eightStatus {
				t.Errorf("-jobs 1 -json reported %d\n%s\n-jobs 8 -json reported %d\n%s", oneStatus, one, eightStatus, eight)
			}
		})
	}
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
