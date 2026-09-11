package analysis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// doubleRow runs the fixture's calc once per row, in the row's context, with the swept
// parameter bound.
func doubleRow(t *testing.T, f *fixture) runtime.SweepRun {
	t.Helper()
	double := f.symbol(t, "Double")
	return func(ctx *runtime.Context, bindings []runtime.SweepBinding) (runtime.SweepRunResult, error) {
		bound := make(map[string]runtime.Value, len(bindings))
		for _, b := range bindings {
			bound[b.Param] = b.Value
		}
		value, err := ctx.InvokeCalcWith(double, nil, bound, f.pkg)
		if err != nil {
			return runtime.SweepRunResult{}, err
		}
		return runtime.SweepRunResult{Outputs: []runtime.CalcOutputValue{{Name: "result", Value: value}}}, nil
	}
}

// doublePlan sweeps x over 1..3 as the calc's parameter types it.
func doublePlan(t *testing.T, f *fixture, ctx *runtime.Context) runtime.SweepPlan {
	t.Helper()
	plan := runtime.SweepPlan{Ranges: []runtime.SweepRange{{Param: "x", From: intOf(1), To: intOf(3)}}}
	resolved, err := ctx.ResolveSweepPlan(f.symbol(t, "Double"), plan, 0, nil)
	if err != nil {
		t.Fatalf("resolve the plan: %v", err)
	}
	return resolved
}

// sweepIn is the one-job sweep with every row in ctx, as a test of a plan's rows takes it.
func sweepIn(ctx *runtime.Context, target string, plan runtime.SweepPlan, run runtime.SweepRun) (runtime.SweepTable, error) {
	return runtime.RunSweepWith(context.Background(), ctx, target, plan, 0, 1, func(int) (*runtime.Context, error) { return ctx, nil }, run)
}

func TestSweepObservesATable(t *testing.T) {
	f := parseFixture(t)
	ctx := f.context(t)
	q := Question{Kind: Sweep, Subject: "test::Double", Schedule: ctx.Schedule(), Sweep: &SweepAsk{Plan: doublePlan(t, f, ctx), Row: doubleRow(t, f)}}
	plan := answered(t, Default(), f.building(), q, Budget{})
	result := plan.Result
	if result.Engine != SweepEngineName || result.Claim != ClaimTable || result.Strength != Observed {
		t.Fatalf("result %+v, want sweep's observed table", result)
	}
	table := result.Table()
	if table.Target != "test::Double" || len(table.Rows) != 3 || len(table.Params) != 1 || table.Params[0] != "x" {
		t.Fatalf("table %+v, want x over 3 rows", table)
	}
	if len(result.Values) != 3 {
		t.Fatalf("values %+v, want one per row", result.Values)
	}
	for i, row := range table.Rows {
		want := int64(2 * (i + 1))
		if row.Err != nil || len(row.Outputs) != 1 || row.Outputs[0].Value.Const.Int != want {
			t.Fatalf("row %d %+v, want result = %d", i, row, want)
		}
		if row.Context == nil || row.Context == ctx {
			t.Fatalf("row %d ran in %p, want a context of its own", i, row.Context)
		}
		if result.Values[i].Row == nil || result.Values[i].Row.Outputs[0].Value.Const.Int != want || result.Values[i].Name != fmt.Sprintf("row %d", i+1) {
			t.Fatalf("value %d %+v, want row %d", i, result.Values[i], i+1)
		}
	}
	if runs, ok := result.Bounds.Limit("runs"); !ok || runs != ctx.SweepRunBudget() || result.Bounds.Reached() {
		t.Fatalf("bounds %s, want the runtime's runs budget unreached", result.Bounds)
	}
	if result.Workers != 1 || plan.Workers != 1 {
		t.Fatalf("workers %d and %d, want the one the plan built for its rows", result.Workers, plan.Workers)
	}
}

func TestSweepTakesTheBudgetsRuns(t *testing.T) {
	f := parseFixture(t)
	ctx := f.context(t)
	q := Question{Kind: Sweep, Subject: "test::Double", Schedule: ctx.Schedule(), Sweep: &SweepAsk{Plan: doublePlan(t, f, ctx), Row: doubleRow(t, f)}}
	result := answered(t, Default(), f.building(), q, Budget{Runs: 5}).Result
	if len(result.Values) != 3 {
		t.Fatalf("values %+v, want the 3 rows within 5 runs", result.Values)
	}
	if runs, ok := result.Bounds.Limit("runs"); !ok || runs != 5 || result.Bounds.Reached() {
		t.Fatalf("bounds %s, want the budget's 5 runs unreached", result.Bounds)
	}
	_, err := Default().Answer(context.Background(), f.building(), q, Budget{Runs: 2})
	if !errors.Is(err, runtime.ErrSweepBudget) || !strings.Contains(err.Error(), "at most 2 allowed") {
		t.Fatalf("3 rows within 2 runs: %v, want the runtime's refusal of the budget's 2", err)
	}
}

func TestRegistrySweepIsTheRuntimesSweep(t *testing.T) {
	f := parseFixture(t)
	ctx := f.context(t)
	plan, err := Default().Sweep(context.Background(), f.building(), "test::Double", ctx.Schedule(), doublePlan(t, f, ctx), doubleRow(t, f), Budget{}, Auto())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	table := plan.Result.Table()
	direct, err := sweepIn(ctx, "test::Double", doublePlan(t, f, ctx), doubleRow(t, f))
	if err != nil {
		t.Fatalf("direct sweep: %v", err)
	}
	if len(table.Rows) != len(direct.Rows) || table.Target != direct.Target {
		t.Fatalf("table %+v, want the runtime's own %+v", table, direct)
	}
	for i := range direct.Rows {
		if table.Rows[i].Outputs[0].Value.Const.Int != direct.Rows[i].Outputs[0].Value.Const.Int {
			t.Fatalf("row %d %+v, want %+v", i, table.Rows[i], direct.Rows[i])
		}
	}
}

func TestSweepRefusalIsTheRuntimes(t *testing.T) {
	f := parseFixture(t)
	ctx := f.context(t)
	plan := runtime.SweepPlan{Ranges: []runtime.SweepRange{{Param: "x", From: intOf(1), To: intOf(4), Step: intOf(0), HasStep: true}}}
	_, err := Default().Sweep(context.Background(), f.building(), "test::Double", ctx.Schedule(), plan, doubleRow(t, f), Budget{}, Auto())
	if !errors.Is(err, runtime.ErrSweepRange) {
		t.Fatalf("sweep with a zero step: %v, want the runtime's range refusal", err)
	}
	_, direct := sweepIn(ctx, "test::Double", plan, doubleRow(t, f))
	if direct == nil || err.Error() != direct.Error() {
		t.Fatalf("sweep: %v, want the runtime's own %v", err, direct)
	}
}

func TestSweepRefusesWhatItCannotRun(t *testing.T) {
	e := NewSweep()
	ask := &SweepAsk{Row: func(*runtime.Context, []runtime.SweepBinding) (runtime.SweepRunResult, error) {
		return runtime.SweepRunResult{}, nil
	}}
	if c := e.Covers(nil, Question{Kind: Evaluate}); c.Covered || !errors.Is(c.Refusal, ErrNotAsked) {
		t.Fatalf("evaluate: %+v, want not asked", c)
	}
	if c := e.Covers(nil, Question{Kind: Sweep, Free: FreeSchedule, Sweep: ask}); c.Covered || !errors.Is(c.Refusal, ErrFreedom) {
		t.Fatalf("free schedule: %+v, want a freedom refusal", c)
	}
	if c := e.Covers(nil, Question{Kind: Sweep}); c.Covered || !errors.Is(c.Refusal, ErrMalformedQuestion) {
		t.Fatalf("no ask: %+v, want malformed", c)
	}
	if c := e.Covers(nil, Question{Kind: Sweep, Sweep: &SweepAsk{}}); c.Covered || !errors.Is(c.Refusal, ErrMalformedQuestion) {
		t.Fatalf("no row: %+v, want malformed", c)
	}
	if c := e.Covers(nil, Question{Kind: Sweep, Sweep: ask}); !c.Covered {
		t.Fatalf("sweep: %+v, want covered", c)
	}
}
