package sweep

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

func TestRuntimeRobustnessSweep(t *testing.T) {
	t.Run("sweep_over_a_boolean_parameter", testSweepOverABooleanParameter)
	t.Run("sweep_over_a_parameter_typed_by_a_part", testSweepOverAParameterTypedByAPart)
	t.Run("sweep_over_an_integer_parameter_by_a_fraction", testSweepOverAnIntegerParameterByAFraction)
	t.Run("sweep_over_a_real_parameter_by_integers_no_real_holds", testSweepOverARealParameterByIntegersNoRealHolds)
	t.Run("monte_carlo_plan_without_runs", testMonteCarloPlanWithoutRuns)
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
		resolved, err := ResolvePlan(ctx, sym, p, 0, nil)
		if err != nil {
			t.Fatalf("%s refused the plan's parameter: %v", name, err)
		}
		runs := 0
		table, err := sweepIn(ctx, context.Background(), "test::"+name, resolved, 0, func(*runtime.Context, []SweepBinding) (SweepRunResult, error) {
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
		From: runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 0}},
		To:   runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}}}}
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
		From: runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 0}},
		To:   runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}}}}
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
	realVal := func(f float64) runtime.Value {
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: f}}
	}
	ctx, scope := analysisFixture(t, sweepRobustnessModel)
	sym, _ := scope.LookupLocal("Sq")
	plan, err := ResolvePlan(ctx, sym, SweepPlan{Ranges: []SweepRange{{
		Param: "x", From: realVal(1), To: realVal(3), Step: realVal(0.5), HasStep: true,
	}}}, 0, nil)
	if err != nil {
		t.Fatalf("resolving x: %v", err)
	}
	_, err = sweepIn(ctx, context.Background(), "test::Sq", plan, 0, func(*runtime.Context, []SweepBinding) (SweepRunResult, error) {
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
	integer := func(n int64) runtime.Value {
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
	}
	const big = int64(1) << 60
	for _, err := range refusedSweepPlan(t, "Half", SweepPlan{Ranges: []SweepRange{{Param: "x", From: integer(big), To: integer(big + 3)}}}) {
		if msg := err.Error(); !strings.Contains(msg, "x : Real") || !strings.Contains(msg, "1152921504606846979") {
			t.Errorf("error = %v, want it to name x : Real and the end 1152921504606846979", err)
		}
	}
	ctx, scope := analysisFixture(t, sweepRobustnessModel)
	sym, _ := scope.LookupLocal("Half")
	plan, err := ResolvePlan(ctx, sym, SweepPlan{Ranges: []SweepRange{{
		Param: "x", From: integer(big), To: integer(big + 512),
	}}}, 0, nil)
	if err != nil {
		t.Fatalf("resolving x: %v", err)
	}
	_, err = sweepIn(ctx, context.Background(), "test::Half", plan, 0, func(*runtime.Context, []SweepBinding) (SweepRunResult, error) {
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
	one := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	plans := map[string]SweepPlan{
		"none":     MonteCarloPlan(0, 1),
		"negative": MonteCarloPlan(-1, 1),
		"ranged":   {Runs: 2, Ranges: []SweepRange{{Param: "x", From: one, To: one}}},
		"sampled":  {Runs: 2, Sampled: true, Samples: 2},
	}
	for name, plan := range plans {
		runs := 0
		table, err := sweepIn(ctx, context.Background(), "test::Sq", plan, 0, func(*runtime.Context, []SweepBinding) (SweepRunResult, error) {
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

// sweepRobustnessModel declares parameters no numeric range can bind, an
// Integer one a fractional step cannot step, and a Real one.
const sweepRobustnessModel = `package test {
	private import ScalarValues::*;
	part def Ship;
	calc def Flag { in b : Boolean; return : Boolean = b; }
	calc def Hull { in s : Ship; return : Ship = s; }
	calc def Sq { in x : Integer; return : Integer = x * x; }
	calc def Half { in x : Real; return : Real = x / 2.0; }
}`
