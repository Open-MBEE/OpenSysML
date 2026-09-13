package main

import (
	"encoding/json"
	"errors"
	"os"
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
