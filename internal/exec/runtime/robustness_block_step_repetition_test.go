package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessBlockStepRepetition covers the failure modes and the
// exploration of an action usage declared `[n]` in a loop or branch body: a body
// stating no succession that orders every performance is refused as open order,
// a loop repeating a counted step without end stops on the action step budget,
// and the interleavings of an inherited count are explored, checked and replayed.
func TestRuntimeRobustnessBlockStepRepetition(t *testing.T) {
	t.Run("unordered_repeated_step_in_body_is_refused_as_open_order", testUnorderedRepeatedBodyStepRefused)
	t.Run("action_step_budget_runs_out_over_repeated_body_steps", testActionStepBudgetOverRepeatedBodySteps)
	t.Run("inherited_count_in_body_is_explored_checked_and_replayed", testInheritedBodyCountExplored)
}

func testUnorderedRepeatedBodyStepRefused(t *testing.T) {
	_, err := executeActionSource(t, "loops", `package P {
		private import ScalarValues::*;
		action loops {
			attribute c : Integer = 0;
			attribute i : Integer = 0;
			while i < 1 {
				action a[2] {
					attribute t : Integer := c;
					assign c := t + 1;
				}
				action bump { assign i := i + 1; }
			}
		}
	}`)
	var sme *lower.StepMultiplicityError
	if !errors.As(err, &sme) {
		t.Fatalf("error = %v, want a StepMultiplicityError", err)
	}
	if sme.Code != lower.StepOrderOpenCode || sme.Step != "a" {
		t.Errorf("error code = %q for step %q, want %q for a: %v", sme.Code, sme.Step, lower.StepOrderOpenCode, err)
	}
}

func testActionStepBudgetOverRepeatedBodySteps(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		action spins {
			attribute c : Integer = 0;
			first start then worker;
			action worker {
				while true {
					first start then a;
					action a[2] { assign c := c + 1; }
					succession first [*] a then [1] pass;
					action pass;
				}
			}
			then done;
		}
	}`))
	ctx.maxActionSteps = 200
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "spins", ast.DefAction)
	if sym == nil {
		t.Fatal("action spins not found")
	}
	err := runWithWatchdog(t, func() error {
		_, err := ctx.ExecuteAction(sym)
		return err
	})
	if !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("error = %v, want ErrActionStepLimitExceeded", err)
	}
}

func testInheritedBodyCountExplored(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		action def Base {
			attribute c : Integer = 0;
			first start then worker;
			action worker {
				attribute i : Integer = 0;
				while i < 1 {
					first start then a;
					action a[2] {
						attribute t : Integer := c;
						assign c := t + 1;
					}
					succession first [*] a then [1] bump;
					action bump { assign i := i + 1; }
				}
			}
			then done;
		}
		action def Derived :> Base { action :>> worker; }
	}`)
	x := m.exploreAction(t, "explore", "Derived")
	if !x.Complete() {
		t.Fatalf("exploration incomplete: %s", x.Status())
	}
	if len(x.Notes) != 0 {
		t.Errorf("notes = %v, want none", x.Notes)
	}
	if len(x.Outcomes) != 2 {
		t.Errorf("outcomes = %v, want c = 1 and c = 2", outcomeTexts(x))
	}
	start := starterOf(m.action(t, "Derived"))
	report := checkStart(t, m, start, unreduced())
	if report.Verdict != CheckDivergent {
		t.Fatalf("check verdict = %q, want divergent over c", report.Verdict)
	}
	replayed := map[string]bool{}
	for _, d := range report.Divergent {
		if d.Feature != "c" {
			continue
		}
		for _, v := range d.Values {
			r := replayWitness(t, m, start, v.Witness, "c = "+v.Value)
			if got := replayedValue(r, "c"); got != v.Value {
				t.Errorf("replaying the witness of c = %s reaches c = %s", v.Value, got)
			}
			replayed[v.Value] = true
		}
	}
	if !replayed["1"] || !replayed["2"] {
		t.Errorf("replayed witnesses for c = %v, want 1 and 2", replayed)
	}
}
