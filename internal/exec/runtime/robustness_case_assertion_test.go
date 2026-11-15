package runtime

import (
	"errors"
	"strings"
	"testing"
)

// caseAssertionVerdicts runs the case named fqn in src and reports its assertion
// verdicts by name beside the run's error.
func caseAssertionVerdicts(t *testing.T, src, fqn string) (map[string]AnalysisVerdict, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	result, err := ctx.RunAnalysis(oneSymbol(t, idx, fqn), AnalysisArgs{}, nil, nil)
	verdicts := make(map[string]AnalysisVerdict, len(result.Verdicts))
	for _, verdict := range result.Verdicts {
		if verdict.Kind == "assertion" {
			verdicts[verdict.Name] = verdict
		}
	}
	return verdicts, err
}

// TestRuntimeRobustnessCaseAssertion: an assertion a case's successions order is
// judged where the flow reaches it, and the ways that can go wrong stay typed.
func TestRuntimeRobustnessCaseAssertion(t *testing.T) {
	t.Parallel()
	t.Run("unreached_assertion_is_undecided", inParallel(func(t *testing.T) {
		verdicts, err := caseAssertionVerdicts(t, `package test {
			private import ScalarValues::*;
			analysis branch {
				attribute level : Integer := 1;
				action low { assign level := 0; }
				assert constraint high { level > 5 }
				first start;
				succession first start then choose;
				succession first high then done;
				succession first low then done;
				decide choose;
				if level > 5 then high;
				if level <= 5 then low;
				return r : Integer = level;
			}
		}`, "test::branch")
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if got := verdicts["high"]; got.Status != VerdictUndecided || got.Detail != unreachedAssertion {
			t.Fatalf("high = %v (%q), want undecided: the flow never reached it", got.Status, got.Detail)
		}
	}))
	t.Run("repeated_assertion_reports_its_first_failure", inParallel(func(t *testing.T) {
		verdicts, err := caseAssertionVerdicts(t, `package test {
			private import ScalarValues::*;
			analysis loop {
				attribute level : Integer := 0;
				action raise { assign level := level + 1; }
				assert constraint low { level < 2 }
				first start;
				merge again;
				succession first start then again;
				succession first again then raise;
				succession first raise then low;
				succession first low then choose;
				decide choose;
				if level < 3 then again;
				if level >= 3 then done;
				return r : Integer = level;
			}
		}`, "test::loop")
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}
		if got := verdicts["low"]; got.Status != VerdictNotSatisfied || got.Detail != "level < 2" {
			t.Fatalf("low = %v (%q), want not satisfied: the second pass violated it", got.Status, got.Detail)
		}
	}))
	t.Run("unevaluable_assertion_is_undecided_and_the_flow_moves_on", inParallel(func(t *testing.T) {
		verdicts, err := caseAssertionVerdicts(t, `package test {
			private import ScalarValues::*;
			analysis early {
				attribute level : Integer := 4;
				assert constraint ratio { 8 / (level - 4) > 1 }
				action shift { assign level := 6; }
				first start then ratio;
				first ratio then shift;
				first shift then done;
				return r : Integer = level;
			}
		}`, "test::early")
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}
		got := verdicts["ratio"]
		if got.Status != VerdictUndecided || !strings.Contains(got.Detail, ErrDivisionByZero.Error()) {
			t.Fatalf("ratio = %v (%q), want undecided by the division by zero at its position", got.Status, got.Detail)
		}
	}))
	t.Run("nested_action_assertion_still_ends_the_run", inParallel(func(t *testing.T) {
		verdicts, err := caseAssertionVerdicts(t, `package test {
			private import ScalarValues::*;
			analysis outer {
				attribute level : Integer := 0;
				action step {
					action raise { assign level := 1; }
					assert constraint raised { level == 3 }
					first start then raise;
					first raise then raised;
					first raised then done;
				}
				return r : Integer = level;
			}
		}`, "test::outer")
		var violation *ViolationError
		if !errors.As(err, &violation) || violation.Element != "raised" {
			t.Fatalf("error = %v, want the step's assertion raised violated", err)
		}
		if _, judged := verdicts["raised"]; judged {
			t.Fatalf("verdicts = %v, want none for an assertion the case does not own", verdicts)
		}
	}))
	t.Run("verification_case_judges_at_position", inParallel(func(t *testing.T) {
		verdicts, err := caseAssertionVerdicts(t, `package test {
			private import ScalarValues::*;
			verification def Check {
				attribute level : Integer := 0;
				action raise { assign level := 3; }
				then assert constraint raised { level == 3 }
				then action lower { assign level := 1; }
				then assert constraint lowered { level == 1 }
			}
			verification check : Check;
		}`, "test::check")
		if err != nil {
			t.Fatalf("run failed: %v", err)
		}
		for _, name := range []string{"raised", "lowered"} {
			if got := verdicts[name]; got.Status != VerdictSatisfied {
				t.Errorf("%s = %v (%q), want satisfied where the flow reached it", name, got.Status, got.Detail)
			}
		}
		if len(verdicts) != 2 {
			t.Errorf("verdicts = %v, want one per assertion", verdicts)
		}
	}))
}
