package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// gateModel: the requirement holds at limit's default and fails for other values
// of it; the assumption admits only values where it holds.
const gateModel = `package Gate {
	private import ScalarValues::*;
	action def open {
		attribute n : Integer = 1;
		attribute limit : Integer = 5;
		requirement positive { require constraint { n + limit > 0 } }
		constraint wide { limit >= 0 and limit < 100 }
		first start;
		action add { assign n := n + 1; }
		done;
		succession first start then add;
		succession first add then done;
	}
}
`

// needsSolver skips a test without an SMT solver installed, or fails it under
// OPENSYSML_REQUIRE_SMT.
func needsSolver(t *testing.T) {
	t.Helper()
	if _, err := solve.Discover(); err != nil {
		if !errors.Is(err, solve.ErrNoSolver) {
			t.Fatalf("discover a solver: %v", err)
		}
		if os.Getenv("OPENSYSML_REQUIRE_SMT") != "" {
			t.Fatalf("OPENSYSML_REQUIRE_SMT is set but %v", err)
		}
		t.Skipf("no SMT solver installed: %v", err)
	}
}

// plannedReport is the plan the JSON report carries beside a check.
type plannedReport struct {
	Checks []struct {
		Status string `json:"status"`
		Plan   *struct {
			Steps []struct {
				Engine string `json:"engine"`
				Status string `json:"status"`
				Detail string `json:"detail"`
			} `json:"steps"`
		} `json:"plan"`
	} `json:"checks"`
}

// planSteps are the engines in the plan of the one check the report carries,
// each with how it took part.
func planSteps(t *testing.T, got runOutcome) map[string]string {
	t.Helper()
	var report plannedReport
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if len(report.Checks) != 1 || report.Checks[0].Plan == nil {
		t.Fatalf("checks %d, want one with a plan\n%s", len(report.Checks), got.output())
	}
	steps := map[string]string{}
	for _, step := range report.Checks[0].Plan.Steps {
		steps[step.Engine] = step.Status
	}
	return steps
}

// A setting the smt engine alone reads, with no property named, puts the action to
// smt under -engine all: released inputs and assumptions beside check's refusal,
// the unroll bound beside check's own answer.
func TestEngineAllPutsSMTOnlySettingsToSMT(t *testing.T) {
	needsSolver(t)
	binary := buildCLI(t)

	for _, tc := range []struct {
		name  string
		flags []string
		check string
	}{
		{"input", []string{"-check-input", "limit"}, "refused"},
		{"assume", []string{"-check-assume", "Gate::open::wide"}, "refused"},
		{"unroll", []string{"-check-unroll", "2"}, "answered"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"-json", "-engine", "all", "-action", "Gate::open"}, tc.flags...)
			got := check(t, binary, gateModel, args...)
			steps := planSteps(t, got)
			if got.status != 0 || steps["smt"] != "answered" || steps["check"] != tc.check {
				t.Errorf("status %d, plan %v; want smt answered and check %s\n%s", got.status, steps, tc.check, got.output())
			}
			if _, explored := steps["explore"]; explored {
				t.Errorf("explore took part in a holds question: %v", steps)
			}
		})
	}

	got := check(t, binary, gateModel, "-engine", "all", "-action", "Gate::open", "-check-input", "limit")
	wantReport(t, got, 0, "inputs: n = 1, limit : Integer free",
		"all: check refused (check cannot leave the inputs free), smt holds (proved)")
}

// sensitivityReport is the paired witness the JSON report carries for a sensitivity.
type sensitivityReport struct {
	Checks []struct {
		Status  string `json:"status"`
		Results []struct {
			Engine   string `json:"engine"`
			Claim    string `json:"claim"`
			Strength string `json:"strength"`
			Reason   string `json:"reason"`
			Witness  *struct {
				Schedule string   `json:"schedule"`
				Choices  []string `json:"choices"`
				Path     string   `json:"path"`
			} `json:"witness"`
			Contrast *struct {
				Schedule string   `json:"schedule"`
				Choices  []string `json:"choices"`
				Path     string   `json:"path"`
			} `json:"contrast"`
		} `json:"results"`
	} `json:"checks"`
}

// -engine smt -check-diverge asks the two-copy query: a feature two branches write is
// sensitive, with two witnesses -check-witness writes apart and -schedule replay:<file>
// follows to the two values; a feature the schedules agree on is not sensitive, at
// proved; a depth short of completion bounds the negative rather than proving it.
func TestEngineSMTDecidesSensitivity(t *testing.T) {
	needsSolver(t)
	binary := buildCLI(t)
	dir := t.TempDir()
	fileA, fileB := filepath.Join(dir, "Mission.race-x-A.witness"), filepath.Join(dir, "Mission.race-x-B.witness")

	got := check(t, binary, forkModel, "-engine", "smt", "-action", "Mission::race", "-check-diverge", "x", "-check-witness", dir)
	wantReport(t, got, 1,
		"✗ Action Mission::race: sensitive: x ends as 1 or 2; the schedules part at step 3:",
		"witness A: "+fileA, "witness B: "+fileB,
		"standing: sensitive (witnessed: witness of 1 choice replayed, inputs as written)")
	for _, file := range []string{fileA, fileB} {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(content), "step 3: ") || !strings.Contains(string(content), "\n\neval literal 0 -> 0\nstep 1: token 1@split\n") {
			t.Errorf("witness %s is not the schedule's choices, a blank line, then the trace:\n%s", file, content)
		}
	}
	values := map[string]bool{}
	for _, file := range []string{fileA, fileB} {
		replayed := check(t, binary, forkModel, "-schedule", "replay:"+file, "-action", "Mission::race")
		wantReport(t, replayed, 0, "standing: value (observed: 1 run under replay:"+file+")")
		for _, value := range []string{"x = 1", "x = 2"} {
			if strings.Contains(replayed.stdout, value) {
				values[value] = true
			}
		}
	}
	if len(values) != 2 {
		t.Errorf("the two witnesses replay to %v, want both values", values)
	}

	got = check(t, binary, forkModel, "-json", "-engine", "smt", "-action", "Mission::race", "-check-diverge", "x", "-check-witness", dir)
	var report sensitivityReport
	if err := json.Unmarshal([]byte(got.stdout), &report); err != nil {
		t.Fatalf("stdout is not the reported JSON: %v\n%s", err, got.output())
	}
	if len(report.Checks) != 1 || len(report.Checks[0].Results) != 1 {
		t.Fatalf("checks %d\n%s", len(report.Checks), got.output())
	}
	r := report.Checks[0].Results[0]
	if r.Engine != "smt" || r.Claim != "sensitive" || r.Strength != "witnessed" || !strings.HasPrefix(r.Reason, "x ends as 1 or 2") {
		t.Errorf("result %+v, want smt's witnessed sensitivity\n%s", r, got.stdout)
	}
	if r.Witness == nil || r.Contrast == nil || r.Witness.Path != fileA || r.Contrast.Path != fileB ||
		r.Witness.Schedule != "replay" || r.Contrast.Schedule != "replay" || len(r.Witness.Choices) != 1 || len(r.Contrast.Choices) != 1 ||
		r.Witness.Choices[0] == r.Contrast.Choices[0] {
		t.Errorf("witness %+v contrast %+v, want the two schedules with their files\n%s", r.Witness, r.Contrast, got.stdout)
	}

	// A feature no branch writes apart is not sensitive, and the negative is a proof.
	agreed := strings.Replace(forkModel, "attribute x : Integer = 0;", "attribute x : Integer = 0;\n        attribute y : Integer = 0;", 1)
	wantReport(t, check(t, binary, agreed, "-engine", "smt", "-action", "Mission::race", "-check-diverge", "y"),
		0, "✓ Action Mission::race: holds", "standing: holds (proved over schedules: inputs as written)")

	// Short of the moves the action needs, no sensitivity is found within the bound; nothing is proved.
	wantReport(t, check(t, binary, forkModel, "-engine", "smt", "-action", "Mission::race", "-check-diverge", "x", "-check-depth", "3"),
		2, "? Action Mission::race: holds (bounded)", "no sensitivity found within 3 moves: a schedule is still live after move 3",
		"standing: holds (bounded over schedules: inputs as written, moves=3 (reached))")

	// The performing object's features are not encoded yet: refused by name, not narrowed away.
	wantReport(t, check(t, binary, straightTankModel, "-engine", "smt", "-instantiate", "Plant::tank", "-action", "Plant::Tank::overfill Plant::tank", "-check-diverge", "this.level"),
		2, "? Action Plant::Tank::overfill: not covered", "this.level: the performing object's features are encoded by a later stage")

	// Beside a feature the encoding answers, a refused one keeps the question not covered:
	// a negative over the list would claim the refused feature too.
	mixed := strings.Replace(straightTankModel, "action overfill {", "action overfill {\n            attribute y : Integer = 0;", 1)
	wantReport(t, check(t, binary, mixed, "-engine", "smt", "-instantiate", "Plant::tank", "-action", "Plant::Tank::overfill Plant::tank", "-check-diverge", "y", "-check-diverge", "this.level"),
		2, "? Action Plant::Tank::overfill: not covered", "standing: not covered (this.level: the performing object's features are encoded by a later stage)")
}

// Under -engine all one Sensitive question reaches check and smt alike, and the
// composition sees both answers about the same feature; the depth bounds both in
// their own unit, as it does a holds question.
func TestEngineAllComposesSensitivity(t *testing.T) {
	needsSolver(t)
	binary := buildCLI(t)

	got := check(t, binary, forkModel, "-json", "-engine", "all", "-action", "Mission::race", "-check-diverge", "x", "-check-depth", "12")
	steps := planSteps(t, got)
	if got.status != 1 || steps["smt"] != "answered" || steps["check"] != "answered" {
		t.Errorf("status %d, plan %v; want check and smt answering\n%s", got.status, steps, got.output())
	}
	if _, explored := steps["explore"]; explored {
		t.Errorf("explore took part in a sensitivity question: %v", steps)
	}
	got = check(t, binary, forkModel, "-engine", "all", "-action", "Mission::race", "-check-diverge", "x", "-check-depth", "12")
	wantReport(t, got, 1, "all: check sensitive (witnessed), smt sensitive (witnessed)")
}
