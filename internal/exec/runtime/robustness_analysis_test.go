package runtime

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestRuntimeRobustnessAnalysis covers trade studies, parameter sweeps and Monte Carlo plans
// that cannot be run as declared.
func TestRuntimeRobustnessAnalysis(t *testing.T) {
	t.Run("trade_study_with_an_abstract_evaluation_function", testTradeStudyWithAnAbstractEvaluationFunction)
	t.Run("trade_study_whose_evaluation_fails_for_one_alternative", testTradeStudyWhoseEvaluationFailsForOneAlternative)
	t.Run("trade_study_with_an_empty_subject", testTradeStudyWithAnEmptySubject)
	t.Run("trade_study_with_a_single_valued_subject", testTradeStudyWithASingleValuedSubject)
	t.Run("trade_study_whose_alternatives_read_an_unbound_feature", testTradeStudyWhoseAlternativesReadAnUnboundFeature)
}

// runTradeStudyExpecting runs the named analysis of tradeStudyRobustnessModel
// and returns what it reported; a run that does not terminate fails the test.
func runTradeStudyExpecting(t *testing.T, name string) (AnalysisResult, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, tradeStudyRobustnessModel))
	ctx.maxSteps = 100000
	scope := idx.DocumentRoot("<test>")
	sym := lookupOne(t, idx, name)

	type outcome struct {
		result AnalysisResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := ctx.RunAnalysis(sym, AnalysisArgs{}, scope, nil)
		done <- outcome{result, err}
	}()
	select {
	case out := <-done:
		if out.err == nil {
			t.Fatalf("%s selected %s, expected the run to fail", name, FormatValue(out.result.Outputs[0].Value))
		}
		return out.result, out.err
	case <-watchdog(10 * time.Second):
		t.Fatalf("%s did not terminate", name)
		return AnalysisResult{}, nil
	}
}

// undecidedObjective asserts the result carries the trade study's objective as
// undecided and returns its detail.
func undecidedObjective(t *testing.T, result AnalysisResult) string {
	t.Helper()
	for _, v := range result.Verdicts {
		if v.Kind == "objective" && v.Name == "tradeStudyObjective" {
			if v.Status != VerdictUndecided {
				t.Fatalf("objective %s = %s, want undecided", v.Name, v.Status)
			}
			return v.Detail
		}
	}
	t.Fatalf("verdicts %v carry no tradeStudyObjective", result.Verdicts)
	return ""
}

// testTradeStudyWithAnAbstractEvaluationFunction: an evaluation function with no
// body computes no value, reported naming the calc, with the objective
// undecided and the one evaluation attempted recorded with its error; no pick
// is fabricated.
func testTradeStudyWithAnAbstractEvaluationFunction(t *testing.T) {
	result, err := runTradeStudyExpecting(t, "test::abstractEval")
	if !errors.Is(err, ErrNoValue) || !strings.Contains(err.Error(), "test::abstractEval::evaluationFunction") {
		t.Fatalf("error = %v, want ErrNoValue naming the evaluation function", err)
	}
	undecidedObjective(t, result)
	if len(result.Outputs) != 0 {
		t.Errorf("outputs = %v, want none from a study that could not evaluate", result.Outputs)
	}
	if len(result.Evaluations) != 1 || result.Evaluations[0].Error == nil || result.Evaluations[0].Selected {
		t.Fatalf("evaluations = %+v, want the one failed attempt and no selection", result.Evaluations)
	}
}

// testTradeStudyWhoseEvaluationFailsForOneAlternative: an alternative whose
// evaluation divides by zero fails the run with that typed error, the objective
// undecided, and the alternatives evaluated before it kept beside the failure.
func testTradeStudyWhoseEvaluationFailsForOneAlternative(t *testing.T) {
	result, err := runTradeStudyExpecting(t, "test::perCylinder")
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("error = %v, want ErrDivisionByZero", err)
	}
	undecidedObjective(t, result)
	if len(result.Evaluations) != 2 {
		t.Fatalf("evaluations = %+v, want light's value and heavy's failure", result.Evaluations)
	}
	if result.Evaluations[0].Error != nil || result.Evaluations[0].Selected || result.Evaluations[0].Tied {
		t.Errorf("light's evaluation = %+v, want a plain value", result.Evaluations[0])
	}
	if !errors.Is(result.Evaluations[1].Error, ErrDivisionByZero) {
		t.Errorf("heavy's evaluation error = %v, want ErrDivisionByZero", result.Evaluations[1].Error)
	}
}

// testTradeStudyWithAnEmptySubject: a subject listing no alternative violates the
// library's [1..*] before anything is evaluated.
func testTradeStudyWithAnEmptySubject(t *testing.T) {
	result, err := runTradeStudyExpecting(t, "test::none")
	if !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "lower bound 1") {
		t.Fatalf("error = %v, want ErrMultiplicityViolation against the lower bound", err)
	}
	undecidedObjective(t, result)
	if len(result.Evaluations) != 0 {
		t.Errorf("evaluations = %+v, want none", result.Evaluations)
	}
}

// testTradeStudyWithASingleValuedSubject: a subject redefined to [1] refuses the
// two alternatives bound to it as a multiplicity violation, not a study of one.
func testTradeStudyWithASingleValuedSubject(t *testing.T) {
	result, err := runTradeStudyExpecting(t, "test::single")
	if !errors.Is(err, ErrMultiplicityViolation) || !strings.Contains(err.Error(), "upper bound 1") {
		t.Fatalf("error = %v, want ErrMultiplicityViolation against the upper bound", err)
	}
	undecidedObjective(t, result)
	if len(result.Evaluations) != 0 {
		t.Errorf("evaluations = %+v, want none", result.Evaluations)
	}
}

// testTradeStudyWhoseAlternativesReadAnUnboundFeature: an evaluation reading a
// feature the first alternative gives no value answers none, which minimize
// reports as the unset feature rather than as a value of the wrong kind; the
// objective is undecided and nothing is selected.
func testTradeStudyWhoseAlternativesReadAnUnboundFeature(t *testing.T) {
	result, err := runTradeStudyExpecting(t, "test::unpriced")
	var noValue *NoValueError
	if !errors.As(err, &noValue) || noValue.Symbol == nil || noValue.Symbol.Name != "cost" {
		t.Fatalf("error = %v, want a NoValueError naming cost", err)
	}
	undecidedObjective(t, result)
	if len(result.Evaluations) != 1 || result.Evaluations[0].Selected {
		t.Fatalf("evaluations = %+v, want heavy's valueless evaluation alone", result.Evaluations)
	}
}
