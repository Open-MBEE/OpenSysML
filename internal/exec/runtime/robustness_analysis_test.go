package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// TestRuntimeRobustnessAnalysis covers trade studies, parameter sweeps and Monte Carlo plans
// that cannot be run as declared.
func TestRuntimeRobustnessAnalysis(t *testing.T) {
	t.Run("trade_study_with_an_abstract_evaluation_function", testTradeStudyWithAnAbstractEvaluationFunction)
	t.Run("trade_study_whose_evaluation_fails_for_one_alternative", testTradeStudyWhoseEvaluationFailsForOneAlternative)
	t.Run("trade_study_with_an_empty_subject", testTradeStudyWithAnEmptySubject)
	t.Run("trade_study_with_a_single_valued_subject", testTradeStudyWithASingleValuedSubject)
	t.Run("trade_study_whose_alternatives_read_an_unbound_feature", testTradeStudyWhoseAlternativesReadAnUnboundFeature)
	t.Run("sweep_over_a_boolean_parameter", testSweepOverABooleanParameter)
	t.Run("sweep_over_a_parameter_typed_by_a_part", testSweepOverAParameterTypedByAPart)
	t.Run("sweep_over_an_integer_parameter_by_a_fraction", testSweepOverAnIntegerParameterByAFraction)
	t.Run("sweep_over_a_real_parameter_by_integers_no_real_holds", testSweepOverARealParameterByIntegersNoRealHolds)
	t.Run("monte_carlo_plan_without_runs", testMonteCarloPlanWithoutRuns)
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
// body is a typed missing-body error naming the calc, with the objective
// undecided and the one evaluation attempted recorded with its error; no pick
// is fabricated.
func testTradeStudyWithAnAbstractEvaluationFunction(t *testing.T) {
	result, err := runTradeStudyExpecting(t, "test::abstractEval")
	if !errors.Is(err, ErrNoResultExpression) || !strings.Contains(err.Error(), "test::abstractEval::evaluationFunction") {
		t.Fatalf("error = %v, want ErrNoResultExpression naming the evaluation function", err)
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

// refusedSweepPlan sweeps the named calc over the plan, both stepped and
// sampled, and returns each typed refusal, failing when a row was run.
func refusedSweepPlan(t *testing.T, name string, plan SweepPlan) []error {
	t.Helper()
	ctx, scope := analysisFixture(t, sweepRobustnessModel)
	sym, ok := scope.LookupLocal(name)
	if !ok {
		t.Fatalf("calc %s not indexed", name)
	}
	sampled := plan
	sampled.Sampled, sampled.Samples, sampled.Seed = true, 2, 1
	var errs []error
	for _, p := range []SweepPlan{plan, sampled} {
		resolved, err := ctx.ResolveSweepPlan(sym, p, 0, nil)
		if err != nil {
			t.Fatalf("%s refused the plan's parameter: %v", name, err)
		}
		runs := 0
		table, err := sweepIn(ctx, context.Background(), "test::"+name, resolved, 0, func(*Context, []SweepBinding) (SweepRunResult, error) {
			runs++
			return SweepRunResult{}, nil
		})
		if err == nil {
			t.Fatalf("%s ran %d row(s); want a refusal", name, len(table.Rows))
		}
		if !errors.Is(err, ErrSweepRange) {
			t.Fatalf("error = %v, want ErrSweepRange", err)
		}
		if runs != 0 {
			t.Fatalf("a refused plan made %d run(s)", runs)
		}
		errs = append(errs, err)
	}
	return errs
}

// testSweepOverABooleanParameter: a Boolean takes no numeric range, so the plan
// is refused naming the parameter and its type, and no row is run.
func testSweepOverABooleanParameter(t *testing.T) {
	plan := SweepPlan{Ranges: []SweepRange{{Param: "b",
		From: Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 0}},
		To:   Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}}}}
	for _, err := range refusedSweepPlan(t, "Flag", plan) {
		if msg := err.Error(); !strings.Contains(msg, "b") || !strings.Contains(msg, "Boolean") {
			t.Errorf("error = %v, want it to name b and Boolean", err)
		}
	}
}

// testSweepOverAParameterTypedByAPart: a part definition is no scalar, so a
// range over a parameter it types is refused before any run.
func testSweepOverAParameterTypedByAPart(t *testing.T) {
	plan := SweepPlan{Ranges: []SweepRange{{Param: "s",
		From: Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 0}},
		To:   Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}}}}
	for _, err := range refusedSweepPlan(t, "Hull", plan) {
		if msg := err.Error(); !strings.Contains(msg, "s") || !strings.Contains(msg, "Ship") {
			t.Errorf("error = %v, want it to name s and Ship", err)
		}
	}
}

// testSweepOverAnIntegerParameterByAFraction: an Integer parameter takes no
// fractional step or endpoint, so the plan is refused rather than half its
// rows failing one by one.
func testSweepOverAnIntegerParameterByAFraction(t *testing.T) {
	realVal := func(f float64) Value {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: f}}
	}
	ctx, scope := analysisFixture(t, sweepRobustnessModel)
	sym, _ := scope.LookupLocal("Sq")
	plan, err := ctx.ResolveSweepPlan(sym, SweepPlan{Ranges: []SweepRange{{
		Param: "x", From: realVal(1), To: realVal(3), Step: realVal(0.5), HasStep: true,
	}}}, 0, nil)
	if err != nil {
		t.Fatalf("resolving x: %v", err)
	}
	_, err = sweepIn(ctx, context.Background(), "test::Sq", plan, 0, func(*Context, []SweepBinding) (SweepRunResult, error) {
		t.Fatal("a row ran under a fractional step")
		return SweepRunResult{}, nil
	})
	if !errors.Is(err, ErrSweepRange) || !strings.Contains(err.Error(), "x : Integer") {
		t.Fatalf("error = %v, want ErrSweepRange naming x : Integer", err)
	}
	for _, err := range refusedSweepPlan(t, "Sq", SweepPlan{Ranges: []SweepRange{{Param: "x", From: realVal(1.5), To: realVal(3)}}}) {
		if !strings.Contains(err.Error(), "x : Integer") {
			t.Errorf("error = %v, want it to name x : Integer", err)
		}
	}
}

// testSweepOverARealParameterByIntegersNoRealHolds: a Real parameter takes the
// Integers of its range as Reals, so endpoints a Real rounds together are
// refused rather than collapsed onto one row, as is a step the reals cannot
// tell apart, before any row runs.
func testSweepOverARealParameterByIntegersNoRealHolds(t *testing.T) {
	integer := func(n int64) Value {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
	}
	const big = int64(1) << 60
	for _, err := range refusedSweepPlan(t, "Half", SweepPlan{Ranges: []SweepRange{{Param: "x", From: integer(big), To: integer(big + 3)}}}) {
		if msg := err.Error(); !strings.Contains(msg, "x : Real") || !strings.Contains(msg, "1152921504606846979") {
			t.Errorf("error = %v, want it to name x : Real and the end 1152921504606846979", err)
		}
	}
	ctx, scope := analysisFixture(t, sweepRobustnessModel)
	sym, _ := scope.LookupLocal("Half")
	plan, err := ctx.ResolveSweepPlan(sym, SweepPlan{Ranges: []SweepRange{{
		Param: "x", From: integer(big), To: integer(big + 512),
	}}}, 0, nil)
	if err != nil {
		t.Fatalf("resolving x: %v", err)
	}
	_, err = sweepIn(ctx, context.Background(), "test::Half", plan, 0, func(*Context, []SweepBinding) (SweepRunResult, error) {
		t.Fatal("a row ran under a step the reals cannot tell apart")
		return SweepRunResult{}, nil
	})
	if !errors.Is(err, ErrSweepRange) || !strings.Contains(err.Error(), "rows would repeat") {
		t.Fatalf("error = %v, want ErrSweepRange refusing repeated rows", err)
	}
}

// testMonteCarloPlanWithoutRuns: a Monte Carlo of fewer than one run, or one
// stating a range or sampling as well, is refused before any run is made.
func testMonteCarloPlanWithoutRuns(t *testing.T) {
	ctx, _ := analysisFixture(t, sweepRobustnessModel)
	one := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	plans := map[string]SweepPlan{
		"none":     MonteCarloPlan(0, 1),
		"negative": MonteCarloPlan(-1, 1),
		"ranged":   {Runs: 2, Ranges: []SweepRange{{Param: "x", From: one, To: one}}},
		"sampled":  {Runs: 2, Sampled: true, Samples: 2},
	}
	for name, plan := range plans {
		runs := 0
		table, err := sweepIn(ctx, context.Background(), "test::Sq", plan, 0, func(*Context, []SweepBinding) (SweepRunResult, error) {
			runs++
			return SweepRunResult{}, nil
		})
		if err == nil {
			t.Fatalf("%s: ran %d row(s); want a refusal", name, len(table.Rows))
		}
		if !errors.Is(err, ErrSweepRuns) {
			t.Fatalf("%s: error = %v, want ErrSweepRuns", name, err)
		}
		if runs != 0 {
			t.Fatalf("%s: a refused plan made %d run(s)", name, runs)
		}
	}
}
