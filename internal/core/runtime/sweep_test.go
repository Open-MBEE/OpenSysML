package runtime

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// sweepModel is the model the sweep tests run over: calcs whose values follow
// straight from their inputs, one of which fails on a particular input.
const sweepModel = `
	package test {
		private import ScalarValues::*;
		calc def Twice {
			in n : Integer;
			return : Integer = n * 2;
		}
		calc def Plus {
			in a : Integer;
			in b : Integer;
			return : Integer = a + b;
		}
		calc def Ratio {
			in a : Real;
			in b : Real;
			return : Real = a / b;
		}
		part def Ship { attribute hullMass : Real; }
		part ship : Ship { attribute :>> hullMass = 1000.0; }
		analysis def Margin {
			subject s : Ship;
			in load : Real;
			in factor : Real;
			return : Real = (s.hullMass + load) * factor;
		}
	}
`

// sweepFixture builds a runtime over the sweep model and returns the package
// scope its calcs are declared in.
func sweepFixture(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	return analysisFixture(t, sweepModel)
}

// calcNamed is the calc definition of that name in the scope.
func calcNamed(t *testing.T, scope *symbols.Scope, name string) *symbols.Symbol {
	t.Helper()
	sym, ok := scope.LookupLocal(name)
	if !ok {
		t.Fatalf("calc %s not indexed", name)
	}
	return sym
}

// intOf and realOf are endpoints as an argument carries them.
func intOf(n int64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
}

func realOf(f float64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: f}}
}

// rangeOf is a range stating no step; steppedRange states one.
func rangeOf(param string, from, to Value) SweepRange {
	return SweepRange{Param: param, From: from, To: to}
}

func steppedRange(param string, from, to, step Value) SweepRange {
	return SweepRange{Param: param, From: from, To: to, Step: step, HasStep: true}
}

// sweepCalcRun makes one ordinary calc run per row with the swept parameters bound.
func sweepCalcRun(ctx *Context, sym *symbols.Symbol, scope *symbols.Scope) SweepRun {
	return func(bindings []SweepBinding) (SweepRunResult, error) {
		bound := make(map[string]Value, len(bindings))
		for _, b := range bindings {
			bound[b.Param] = b.Value
		}
		value, err := ctx.InvokeCalcWith(sym, nil, bound, scope)
		if err != nil {
			return SweepRunResult{}, err
		}
		return SweepRunResult{Outputs: []CalcOutputValue{{Name: "result", Value: value}}}, nil
	}
}

// runSweepOver sweeps the named calc over the plan, failing the test when the
// plan itself was refused.
func runSweepOver(t *testing.T, ctx *Context, scope *symbols.Scope, name string, plan SweepPlan) SweepTable {
	t.Helper()
	sym := calcNamed(t, scope, name)
	table, err := ctx.RunSweep(context.Background(), "test::"+name, plan, sweepCalcRun(ctx, sym, scope))
	if err != nil {
		t.Fatalf("sweep of %s: %v", name, err)
	}
	return table
}

// refuseSweep sweeps and reports the refusal, failing the test when the plan
// ran anyway.
func refuseSweep(t *testing.T, ctx *Context, scope *symbols.Scope, name string, plan SweepPlan) error {
	t.Helper()
	sym := calcNamed(t, scope, name)
	table, err := ctx.RunSweep(context.Background(), "test::"+name, plan, sweepCalcRun(ctx, sym, scope))
	if err == nil {
		t.Fatalf("sweep of %s ran %d row(s); want a refusal", name, len(table.Rows))
	}
	if len(table.Rows) != 0 {
		t.Fatalf("refused sweep reported %d row(s); want none", len(table.Rows))
	}
	return err
}

// tableText renders a table as one line per row — its inputs, its outputs and
// its failure — so one comparison covers the rows, their order and their
// values. Times are left out, since they differ from run to run.
func tableText(table SweepTable) string {
	lines := make([]string, 0, len(table.Rows))
	for _, row := range table.Rows {
		parts := make([]string, 0, len(row.Bindings)+len(row.Outputs)+1)
		for _, b := range row.Bindings {
			parts = append(parts, b.Param+"="+FormatValue(b.Value))
		}
		for _, out := range row.Outputs {
			parts = append(parts, out.Name+" -> "+FormatValue(out.Value))
		}
		if row.Err != nil {
			parts = append(parts, "error: "+strings.Join(strings.Fields(row.Err.Error()), " "))
		}
		lines = append(lines, strings.Join(parts, " "))
	}
	return strings.Join(lines, "\n")
}

// inputsOf lists what each row bound, which is what a sampled table is judged
// on.
func inputsOf(table SweepTable) []string {
	values := make([]string, 0, len(table.Rows))
	for _, row := range table.Rows {
		parts := make([]string, 0, len(row.Bindings))
		for _, b := range row.Bindings {
			parts = append(parts, b.Param+"="+FormatValue(b.Value))
		}
		values = append(values, strings.Join(parts, " "))
	}
	return values
}

// A range between Integers stating no step advances by one, from its start
// through its end, and each row is one ordinary run of the calc.
func TestSweepIntegerRangeStepsByOne(t *testing.T) {
	ctx, scope := sweepFixture(t)
	table := runSweepOver(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{rangeOf("n", intOf(1), intOf(4))},
	})
	got := tableText(table)
	want := strings.Join([]string{
		"n=1 result -> 2",
		"n=2 result -> 4",
		"n=3 result -> 6",
		"n=4 result -> 8",
	}, "\n")
	if got != want {
		t.Errorf("table is\n%s\nwant\n%s", got, want)
	}
	if table.Params[0] != "n" || table.Sampled {
		t.Errorf("table reports params %v, sampled %v; want [n], false", table.Params, table.Sampled)
	}
	for i, row := range table.Rows {
		if row.Elapsed < 0 {
			t.Errorf("row %d took %s", i, row.Elapsed)
		}
	}
}

// A range whose end lies against the direction of its Integer default steps
// towards it rather than refusing.
func TestSweepIntegerRangeDescends(t *testing.T) {
	ctx, scope := sweepFixture(t)
	table := runSweepOver(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{rangeOf("n", intOf(2), intOf(0))},
	})
	if got, want := inputsOf(table), []string{"n=2", "n=1", "n=0"}; !equalStrings(got, want) {
		t.Errorf("rows bound %v; want %v", got, want)
	}
}

// A stated step advances by it, and the end is a row where the step lands on it.
func TestSweepStepIncludesEndpointItLandsOn(t *testing.T) {
	ctx, scope := sweepFixture(t)
	on := runSweepOver(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{steppedRange("n", intOf(0), intOf(6), intOf(3))},
	})
	if got, want := inputsOf(on), []string{"n=0", "n=3", "n=6"}; !equalStrings(got, want) {
		t.Errorf("rows bound %v; want %v", got, want)
	}
	past := runSweepOver(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{steppedRange("n", intOf(0), intOf(7), intOf(3))},
	})
	if got, want := inputsOf(past), []string{"n=0", "n=3", "n=6"}; !equalStrings(got, want) {
		t.Errorf("rows bound %v; want %v", got, want)
	}
}

// A real range steps by the step it states, and reaches its end rather than
// stopping a rounding error short of it.
func TestSweepRealRangeReachesItsEnd(t *testing.T) {
	ctx, scope := sweepFixture(t)
	table := runSweepOver(t, ctx, scope, "Ratio", SweepPlan{
		Ranges: []SweepRange{
			steppedRange("a", realOf(0), realOf(1), realOf(0.1)),
			rangeOf("b", intOf(1), intOf(1)),
		},
	})
	if len(table.Rows) != 11 {
		t.Fatalf("0.0..1.0:0.1 ran %d row(s); want 11", len(table.Rows))
	}
	last := table.Rows[len(table.Rows)-1]
	if got := FormatValue(last.Bindings[0].Value); got != "1.0" {
		t.Errorf("last row bound a=%s; want 1.0", got)
	}
}

// Several ranges run their cartesian product, the first parameter given varying
// slowest, so the rows read in the order the ranges were written.
func TestSweepSeveralRangesRunTheirCartesianProduct(t *testing.T) {
	ctx, scope := sweepFixture(t)
	table := runSweepOver(t, ctx, scope, "Plus", SweepPlan{
		Ranges: []SweepRange{
			rangeOf("a", intOf(1), intOf(3)),
			rangeOf("b", intOf(10), intOf(11)),
		},
	})
	got := tableText(table)
	want := strings.Join([]string{
		"a=1 b=10 result -> 11",
		"a=1 b=11 result -> 12",
		"a=2 b=10 result -> 12",
		"a=2 b=11 result -> 13",
		"a=3 b=10 result -> 13",
		"a=3 b=11 result -> 14",
	}, "\n")
	if got != want {
		t.Errorf("table is\n%s\nwant\n%s", got, want)
	}
}

// A run that failed is that row's typed error, and the runs after it are made
// all the same.
func TestSweepFailedRunIsARowAndTheTableGoesOn(t *testing.T) {
	ctx, scope := sweepFixture(t)
	table := runSweepOver(t, ctx, scope, "Ratio", SweepPlan{
		Ranges: []SweepRange{
			rangeOf("a", intOf(4), intOf(4)),
			rangeOf("b", intOf(-1), intOf(1)),
		},
	})
	if len(table.Rows) != 3 {
		t.Fatalf("table has %d row(s); want 3", len(table.Rows))
	}
	failed := table.Rows[1]
	if failed.Err == nil {
		t.Fatalf("dividing by zero produced %s; want a failed row", tableText(table))
	}
	if !errors.Is(failed.Err, semantics.ErrDivisionByZero) {
		t.Errorf("row error = %v; want a division by zero", failed.Err)
	}
	if len(failed.Outputs) != 0 {
		t.Errorf("failed row reported outputs %v; want none", failed.Outputs)
	}
	for _, i := range []int{0, 2} {
		if table.Rows[i].Err != nil || len(table.Rows[i].Outputs) != 1 {
			t.Errorf("row %d = %+v; want one output and no error", i, table.Rows[i])
		}
	}
}

// A real range states its step: no step is the obvious one between two reals,
// so leaving it out is refused rather than guessed at.
func TestSweepRealRangeWithoutAStepIsRefused(t *testing.T) {
	ctx, scope := sweepFixture(t)
	err := refuseSweep(t, ctx, scope, "Ratio", SweepPlan{
		Ranges: []SweepRange{rangeOf("a", realOf(0), realOf(1))},
	})
	if !errors.Is(err, ErrSweepRange) || !strings.Contains(err.Error(), "step") {
		t.Errorf("err = %v; want an ErrSweepRange naming the step", err)
	}
}

// A step of zero never reaches the end of its range.
func TestSweepZeroStepIsRefused(t *testing.T) {
	ctx, scope := sweepFixture(t)
	err := refuseSweep(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{steppedRange("n", intOf(1), intOf(4), intOf(0))},
	})
	if !errors.Is(err, ErrSweepRange) {
		t.Errorf("err = %v; want ErrSweepRange", err)
	}
}

// A step whose sign leads away from the end of its range never reaches it.
func TestSweepStepAwayFromTheEndIsRefused(t *testing.T) {
	ctx, scope := sweepFixture(t)
	err := refuseSweep(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{steppedRange("n", intOf(1), intOf(4), intOf(-1))},
	})
	if !errors.Is(err, ErrSweepRange) {
		t.Errorf("err = %v; want ErrSweepRange", err)
	}
}

// An endpoint that is no number at all states no range.
func TestSweepNonNumericEndpointIsRefused(t *testing.T) {
	ctx, scope := sweepFixture(t)
	err := refuseSweep(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{rangeOf("n",
			Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: true}},
			intOf(4))},
	})
	if !errors.Is(err, ErrSweepRange) {
		t.Errorf("err = %v; want ErrSweepRange", err)
	}
}

// A range between an endpoint carrying a unit and one that carries none
// measures two different things, and is refused.
func TestSweepEndpointsMustBothCarryAUnit(t *testing.T) {
	ctx, scope := sweepFixture(t)
	metre := NewQuantityValue(&Quantity{
		Num:  semantics.Value{Kind: semantics.ValReal, Real: 0},
		Unit: Unit{Text: "SI::m"},
	})
	err := refuseSweep(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{steppedRange("n", metre, realOf(10), realOf(2))},
	})
	if !errors.Is(err, ErrSweepRange) || !strings.Contains(err.Error(), "unit") {
		t.Errorf("err = %v; want an ErrSweepRange naming the unit", err)
	}
}

// A sweep naming no range at all runs nothing.
func TestSweepWithoutARangeIsRefused(t *testing.T) {
	ctx, scope := sweepFixture(t)
	err := refuseSweep(t, ctx, scope, "Twice", SweepPlan{})
	if !errors.Is(err, ErrSweepEmpty) {
		t.Errorf("err = %v; want ErrSweepEmpty", err)
	}
}

// One parameter takes one range: sweeping it twice states two.
func TestSweepParameterSweptTwiceIsRefused(t *testing.T) {
	ctx, scope := sweepFixture(t)
	err := refuseSweep(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{
			rangeOf("n", intOf(1), intOf(2)),
			rangeOf("n", intOf(3), intOf(4)),
		},
	})
	if !errors.Is(err, ErrSweepParameter) {
		t.Errorf("err = %v; want ErrSweepParameter", err)
	}
}

// A sweep asking for more runs than its budget allows is refused before any run
// is made, and says which variable raises the budget.
func TestSweepBudgetIsRefusedBeforeRunning(t *testing.T) {
	ctx, scope := sweepFixture(t)
	err := refuseSweep(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{rangeOf("n", intOf(1), intOf(ctx.SweepRunBudget()+1))},
	})
	if !errors.Is(err, ErrSweepBudget) || !strings.Contains(err.Error(), MaxSweepRunsEnvVar) {
		t.Errorf("err = %v; want an ErrSweepBudget naming %s", err, MaxSweepRunsEnvVar)
	}
}

// The budget bounds the product of the ranges, not each range on its own.
func TestSweepBudgetBoundsTheProduct(t *testing.T) {
	ctx, scope := sweepFixture(t)
	half := ctx.SweepRunBudget()/2 + 1
	err := refuseSweep(t, ctx, scope, "Plus", SweepPlan{
		Ranges: []SweepRange{
			rangeOf("a", intOf(1), intOf(half)),
			rangeOf("b", intOf(1), intOf(3)),
		},
	})
	if !errors.Is(err, ErrSweepBudget) {
		t.Errorf("err = %v; want ErrSweepBudget", err)
	}
}

// A parameter the target declares none of, and one the invocation already binds,
// are both refused before anything runs.
func TestCheckSweepParametersRefusesUnknownAndBoundParameters(t *testing.T) {
	ctx, scope := sweepFixture(t)
	sym := calcNamed(t, scope, "Plus")
	plan := SweepPlan{Ranges: []SweepRange{rangeOf("a", intOf(1), intOf(2))}}

	if err := ctx.CheckSweepParameters(sym, plan, 0, []string{"b"}); err != nil {
		t.Fatalf("sweeping a while b is bound: %v", err)
	}
	err := ctx.CheckSweepParameters(sym, SweepPlan{
		Ranges: []SweepRange{rangeOf("nope", intOf(1), intOf(2))},
	}, 0, nil)
	if !errors.Is(err, ErrSweepParameter) || !strings.Contains(err.Error(), "nope") {
		t.Errorf("err = %v; want an ErrSweepParameter naming nope", err)
	}
	if err := ctx.CheckSweepParameters(sym, plan, 0, []string{"a"}); !errors.Is(err, ErrSweepParameter) {
		t.Errorf("sweeping a parameter bound by name: err = %v; want ErrSweepParameter", err)
	}
	if err := ctx.CheckSweepParameters(sym, plan, 1, nil); !errors.Is(err, ErrSweepParameter) {
		t.Errorf("sweeping a parameter bound by position: err = %v; want ErrSweepParameter", err)
	}
}

// A sampled range draws one value per row per parameter, so the draws pair up
// into rows rather than multiplying into a product.
func TestSamplesDrawOneRowPerDraw(t *testing.T) {
	ctx, scope := sweepFixture(t)
	table := runSweepOver(t, ctx, scope, "Plus", SweepPlan{
		Ranges: []SweepRange{
			rangeOf("a", intOf(0), intOf(100)),
			rangeOf("b", intOf(0), intOf(100)),
		},
		Sampled: true, Samples: 5, Seed: 11,
	})
	if len(table.Rows) != 5 {
		t.Fatalf("5 samples ran %d row(s)", len(table.Rows))
	}
	if !table.Sampled || table.Seed != 11 {
		t.Errorf("table reports sampled %v, seed %d; want true, 11", table.Sampled, table.Seed)
	}
	for _, row := range table.Rows {
		if len(row.Bindings) != 2 || row.Err != nil {
			t.Errorf("row %+v; want both parameters bound and no error", row)
		}
	}
}

// The same seed draws the same table, and a different seed draws a different
// one: the generator is seeded from the request alone.
func TestSamplesAreReproducibleFromTheirSeed(t *testing.T) {
	ctx, scope := sweepFixture(t)
	plan := func(seed uint64) SweepPlan {
		return SweepPlan{
			Ranges:  []SweepRange{rangeOf("n", intOf(0), intOf(1_000_000))},
			Sampled: true, Samples: 8, Seed: seed,
		}
	}
	first := tableText(runSweepOver(t, ctx, scope, "Twice", plan(7)))
	again := tableText(runSweepOver(t, ctx, scope, "Twice", plan(7)))
	other := tableText(runSweepOver(t, ctx, scope, "Twice", plan(8)))
	if first != again {
		t.Errorf("seed 7 drew\n%s\nthen\n%s", first, again)
	}
	if first == other {
		t.Errorf("seed 8 drew what seed 7 did:\n%s", first)
	}
}

// The values a seed draws are pinned, so the same table comes out on every
// platform and every build.
func TestSamplesDrawnValuesArePinned(t *testing.T) {
	ctx, scope := sweepFixture(t)
	table := runSweepOver(t, ctx, scope, "Twice", SweepPlan{
		Ranges:  []SweepRange{rangeOf("n", intOf(0), intOf(999))},
		Sampled: true, Samples: 6, Seed: 42,
	})
	got := strings.Join(inputsOf(table), " ")
	want := "n=454 n=972 n=719 n=345 n=838 n=944"
	if got != want {
		t.Errorf("seed 42 drew %s; want %s", got, want)
	}
}

// A sampled Integer range draws over its endpoints inclusively, so both of a
// range of two values are drawn and nothing outside it is.
func TestSamplesOverIntegersIncludeBothEndpoints(t *testing.T) {
	ctx, scope := sweepFixture(t)
	table := runSweepOver(t, ctx, scope, "Twice", SweepPlan{
		Ranges:  []SweepRange{rangeOf("n", intOf(0), intOf(1))},
		Sampled: true, Samples: 40, Seed: 3,
	})
	seen := map[string]int{}
	for _, row := range table.Rows {
		seen[FormatValue(row.Bindings[0].Value)]++
	}
	if len(seen) != 2 || seen["0"] == 0 || seen["1"] == 0 {
		t.Errorf("draws over 0..1 were %v; want both endpoints and nothing else", seen)
	}
}

// A sampled real range draws over [from, to), and needs no step to do it.
func TestSamplesOverRealsStayWithinTheirRange(t *testing.T) {
	ctx, scope := sweepFixture(t)
	table := runSweepOver(t, ctx, scope, "Ratio", SweepPlan{
		Ranges: []SweepRange{
			rangeOf("a", realOf(1), realOf(2)),
			rangeOf("b", intOf(1), intOf(1)),
		},
		Sampled: true, Samples: 25, Seed: 5,
	})
	for _, row := range table.Rows {
		drawn := row.Bindings[0].Value.Const.AsReal()
		if drawn < 1 || drawn >= 2 {
			t.Errorf("drew a=%v, which is outside [1.0, 2.0)", drawn)
		}
	}
}

// A step is what a swept range advances by; a sampled range draws instead, so
// stating one is refused rather than ignored.
func TestSamplesWithAStepAreRefused(t *testing.T) {
	ctx, scope := sweepFixture(t)
	err := refuseSweep(t, ctx, scope, "Twice", SweepPlan{
		Ranges:  []SweepRange{steppedRange("n", intOf(1), intOf(9), intOf(2))},
		Sampled: true, Samples: 3, Seed: 1,
	})
	if !errors.Is(err, ErrSweepSamples) {
		t.Errorf("err = %v; want ErrSweepSamples", err)
	}
}

// A sample count that draws nothing is refused.
func TestSamplesWithoutADrawAreRefused(t *testing.T) {
	ctx, scope := sweepFixture(t)
	for _, count := range []int64{0, -3} {
		err := refuseSweep(t, ctx, scope, "Twice", SweepPlan{
			Ranges:  []SweepRange{rangeOf("n", intOf(1), intOf(9))},
			Sampled: true, Samples: count, Seed: 1,
		})
		if !errors.Is(err, ErrSweepSamples) {
			t.Errorf("%d samples: err = %v; want ErrSweepSamples", count, err)
		}
	}
}

// The budget bounds a sampled sweep as it bounds a swept one.
func TestSamplesBeyondTheBudgetAreRefused(t *testing.T) {
	ctx, scope := sweepFixture(t)
	err := refuseSweep(t, ctx, scope, "Twice", SweepPlan{
		Ranges:  []SweepRange{rangeOf("n", intOf(1), intOf(9))},
		Sampled: true, Samples: ctx.SweepRunBudget() + 1, Seed: 1,
	})
	if !errors.Is(err, ErrSweepBudget) {
		t.Errorf("err = %v; want ErrSweepBudget", err)
	}
}

// The generator is the one documented: one seed selects one PCG state, and the
// sequence it draws is the same on every platform.
func TestNewSampleSourceIsSeededFromTheSeedAlone(t *testing.T) {
	first, again := NewSampleSource(9), NewSampleSource(9)
	other := NewSampleSource(10)
	for i := range 4 {
		a, b, c := first.Uint64(), again.Uint64(), other.Uint64()
		if a != b {
			t.Fatalf("draw %d from seed 9 is %d then %d", i, a, b)
		}
		if a == c {
			t.Fatalf("draw %d from seed 10 is what seed 9 drew: %d", i, a)
		}
	}
}

// A range between Integers too large for float64 to count by ones through
// still takes every value between its endpoints.
func TestSweepIntegerRangeBeyondFloatPrecisionStepsExactly(t *testing.T) {
	ctx, scope := sweepFixture(t)
	const big = int64(1) << 60
	table := runSweepOver(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{rangeOf("n", intOf(big), intOf(big+3))},
	})
	want := []string{"n=1152921504606846976", "n=1152921504606846977", "n=1152921504606846978", "n=1152921504606846979"}
	if got := inputsOf(table); !equalStrings(got, want) {
		t.Errorf("rows bound %v; want %v", got, want)
	}
	stepped := runSweepOver(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{steppedRange("n", intOf(1<<53), intOf(1<<53+4), intOf(2))},
	})
	wantStepped := []string{"n=9007199254740992", "n=9007199254740994", "n=9007199254740996"}
	if got := inputsOf(stepped); !equalStrings(got, wantStepped) {
		t.Errorf("rows bound %v; want %v", got, wantStepped)
	}
}

// A range against the ends of Integer arithmetic reaches its endpoints rather
// than wrapping past them, in either direction and by a step of any width.
func TestSweepIntegerRangeAtTheIntegerExtremes(t *testing.T) {
	ctx, scope := sweepFixture(t)
	cases := []struct {
		name  string
		plan  SweepRange
		bound []string
	}{
		{"top", rangeOf("n", intOf(math.MaxInt64-2), intOf(math.MaxInt64)),
			[]string{"n=9223372036854775805", "n=9223372036854775806", "n=9223372036854775807"}},
		{"bottom", rangeOf("n", intOf(math.MinInt64), intOf(math.MinInt64+2)),
			[]string{"n=-9223372036854775808", "n=-9223372036854775807", "n=-9223372036854775806"}},
		{"descending", rangeOf("n", intOf(math.MinInt64+2), intOf(math.MinInt64)),
			[]string{"n=-9223372036854775806", "n=-9223372036854775807", "n=-9223372036854775808"}},
		{"widest step", steppedRange("n", intOf(math.MinInt64), intOf(math.MaxInt64), intOf(math.MaxInt64)),
			[]string{"n=-9223372036854775808", "n=-1", "n=9223372036854775806"}},
		{"singleton", rangeOf("n", intOf(math.MaxInt64), intOf(math.MaxInt64)),
			[]string{"n=9223372036854775807"}},
	}
	for _, c := range cases {
		table := runSweepOver(t, ctx, scope, "Twice", SweepPlan{Ranges: []SweepRange{c.plan}})
		if got := inputsOf(table); !equalStrings(got, c.bound) {
			t.Errorf("%s: rows bound %v; want %v", c.name, got, c.bound)
		}
	}
}

// A range spanning every Integer takes more runs than any budget allows, and
// is refused by the budget rather than counted wrongly.
func TestSweepOverEveryIntegerIsRefusedByTheBudget(t *testing.T) {
	ctx, scope := sweepFixture(t)
	err := refuseSweep(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{rangeOf("n", intOf(math.MinInt64), intOf(math.MaxInt64))},
	})
	if !errors.Is(err, ErrSweepBudget) {
		t.Errorf("err = %v; want ErrSweepBudget", err)
	}
}

// Sampling a range float64 cannot count through draws Integers spread over it,
// rather than collapsing onto one endpoint.
func TestSamplesOverWideIntegerRangesSpreadOverThem(t *testing.T) {
	ctx, scope := sweepFixture(t)
	const big = int64(1) << 60
	cases := []struct {
		name     string
		from, to int64
	}{
		{"beyond float precision", big, big + 4},
		{"every integer", math.MinInt64, math.MaxInt64},
	}
	for _, c := range cases {
		table := runSweepOver(t, ctx, scope, "Twice", SweepPlan{
			Ranges:  []SweepRange{rangeOf("n", intOf(c.from), intOf(c.to))},
			Sampled: true, Samples: 12, Seed: 11,
		})
		seen := map[int64]bool{}
		for _, row := range table.Rows {
			drawn := row.Bindings[0].Value.Const.Int
			if drawn < c.from || drawn > c.to {
				t.Errorf("%s: drew n=%d, which is outside %d..%d", c.name, drawn, c.from, c.to)
			}
			seen[drawn] = true
		}
		if len(seen) < 2 {
			t.Errorf("%s: 12 draws took %d value(s); want a spread", c.name, len(seen))
		}
	}
}

// A sweep stops between runs once its caller has gone away, and reports why
// rather than a table.
func TestSweepStopsWhenItsCallerGoesAway(t *testing.T) {
	ctx, scope := sweepFixture(t)
	sym := calcNamed(t, scope, "Twice")
	run := sweepCalcRun(ctx, sym, scope)
	stop, cancel := context.WithCancel(context.Background())
	defer cancel()
	ran := 0
	counted := func(bindings []SweepBinding) (SweepRunResult, error) {
		ran++
		if ran == 2 {
			cancel()
		}
		return run(bindings)
	}
	table, err := ctx.RunSweep(stop, "test::Twice", SweepPlan{
		Ranges: []SweepRange{rangeOf("n", intOf(1), intOf(20))},
	}, counted)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v; want context.Canceled", err)
	}
	if ran != 2 {
		t.Errorf("%d run(s) were made; want the sweep to stop after the second", ran)
	}
	if len(table.Rows) != 0 {
		t.Errorf("a stopped sweep reported %d row(s); want none", len(table.Rows))
	}
}

// quantityInt is an Integer magnitude in the given unit, as an argument carries it.
func quantityInt(n int64, unit string, scale float64) Value {
	return NewQuantityValue(&Quantity{
		Num:  semantics.Value{Kind: semantics.ValInt, Int: n},
		Unit: Unit{Text: unit, Term: semantics.UnitTerm{Scale: semantics.UnitScale(scale)}},
	})
}

// An Integer range carrying a unit keeps its endpoints exactly past the
// magnitudes float64 counts by ones through, so it still steps by one.
func TestSweepQuantityIntegerRangeBeyondFloatPrecisionStepsExactly(t *testing.T) {
	ctx, scope := sweepFixture(t)
	base := int64(1) << 60
	table := runSweepOver(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{rangeOf("n",
			quantityInt(base, "SI::m", 1), quantityInt(base+2, "SI::m", 1))},
	})
	want := []int64{base, base + 1, base + 2}
	if len(table.Rows) != len(want) {
		t.Fatalf("rows = %d; want %d", len(table.Rows), len(want))
	}
	for i, n := range want {
		q := table.Rows[i].Bindings[0].Value.Quantity()
		if q == nil || q.Num.Kind != semantics.ValInt || q.Num.Int != n {
			t.Errorf("row %d bound %s; want %d [SI::m]", i, FormatValue(table.Rows[i].Bindings[0].Value), n)
		}
	}
}

// A range whose endpoints are expressed in units a whole factor apart converts
// them exactly, so it stays a range between Integers past that precision too.
func TestSweepQuantityRangeConvertsIntegerEndpointsExactly(t *testing.T) {
	ctx, scope := sweepFixture(t)
	const kilometres = 1152921504606844
	metres := int64(kilometres) * 1000
	table := runSweepOver(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{steppedRange("n",
			quantityInt(metres, "SI::m", 1),
			quantityInt(kilometres+2, "SI::km", 1000),
			quantityInt(1000, "SI::m", 1))},
	})
	want := []int64{metres, metres + 1000, metres + 2000}
	if len(table.Rows) != len(want) {
		t.Fatalf("rows = %d; want %d", len(table.Rows), len(want))
	}
	for i, n := range want {
		q := table.Rows[i].Bindings[0].Value.Quantity()
		if q == nil || q.Num.Kind != semantics.ValInt || q.Num.Int != n {
			t.Errorf("row %d bound %s; want %d [SI::m]", i, FormatValue(table.Rows[i].Bindings[0].Value), n)
		}
	}
}

// A case's positional arguments bind the inputs its run binds them to, the
// subject skipped, so a parameter one of them binds cannot also be swept.
func TestCheckSweepParametersFollowsAnalysisPositionalBinding(t *testing.T) {
	ctx, scope := sweepFixture(t)
	sym := calcNamed(t, scope, "Margin")

	load := SweepPlan{Ranges: []SweepRange{rangeOf("load", realOf(0), realOf(1))}}
	if err := ctx.CheckSweepParameters(sym, load, 1, nil); !errors.Is(err, ErrSweepParameter) {
		t.Errorf("sweeping the input the positional argument binds: err = %v; want ErrSweepParameter", err)
	}
	factor := SweepPlan{Ranges: []SweepRange{rangeOf("factor", realOf(1), realOf(2))}}
	if err := ctx.CheckSweepParameters(sym, factor, 1, nil); err != nil {
		t.Errorf("sweeping the input no argument binds: %v", err)
	}
	if err := ctx.CheckSweepParameters(sym, factor, 2, nil); !errors.Is(err, ErrSweepParameter) {
		t.Errorf("sweeping the second input two arguments bind: err = %v; want ErrSweepParameter", err)
	}
}

// A case's subject is an object an instantiation binds, which no range of
// values stands for, so sweeping it is refused rather than run.
func TestCheckSweepParametersRefusesTheSubject(t *testing.T) {
	ctx, scope := sweepFixture(t)
	sym := calcNamed(t, scope, "Margin")
	err := ctx.CheckSweepParameters(sym, SweepPlan{
		Ranges: []SweepRange{rangeOf("s", intOf(1), intOf(2))},
	}, 0, nil)
	if !errors.Is(err, ErrSweepParameter) || !strings.Contains(err.Error(), "subject") {
		t.Errorf("err = %v; want an ErrSweepParameter naming the subject", err)
	}
}

// A sampled range carrying a unit draws Integers over the whole of it, past the
// magnitudes float64 counts by ones through.
func TestSamplesOverAQuantityIntegerRangeStayIntegers(t *testing.T) {
	ctx, scope := sweepFixture(t)
	base := int64(1) << 60
	table := runSweepOver(t, ctx, scope, "Twice", SweepPlan{
		Ranges: []SweepRange{rangeOf("n",
			quantityInt(base, "SI::m", 1), quantityInt(base+1000, "SI::m", 1))},
		Sampled: true, Samples: 16, Seed: 7,
	})
	spread := make(map[int64]bool, len(table.Rows))
	for i := range table.Rows {
		q := table.Rows[i].Bindings[0].Value.Quantity()
		if q == nil || q.Num.Kind != semantics.ValInt {
			t.Fatalf("row %d drew %s; want an Integer quantity",
				i, FormatValue(table.Rows[i].Bindings[0].Value))
		}
		if q.Num.Int < base || q.Num.Int > base+1000 {
			t.Errorf("row %d drew %d; want a value in the range", i, q.Num.Int)
		}
		spread[q.Num.Int] = true
	}
	if len(spread) < 2 {
		t.Errorf("16 draws took %d distinct value(s); want them spread over the range", len(spread))
	}
}

// A range whose endpoint or step is not a finite number states no run to make,
// stepped or drawn, so it is refused before any row is counted.
func TestSweepNonFiniteRangeIsRefused(t *testing.T) {
	ctx, scope := sweepFixture(t)
	quantity := func(m float64) Value {
		return NewQuantityValue(&Quantity{
			Num:  semantics.Value{Kind: semantics.ValReal, Real: m},
			Unit: Unit{Text: "SI::m", Term: semantics.UnitTerm{Scale: semantics.UnitScale(1)}},
		})
	}
	kilometre := NewQuantityValue(&Quantity{
		Num:  semantics.Value{Kind: semantics.ValReal, Real: 1},
		Unit: Unit{Text: "SI::km", Term: semantics.UnitTerm{Scale: semantics.UnitScale(1000)}},
	})
	cases := []struct {
		name string
		plan SweepPlan
	}{
		{"start is not a number", SweepPlan{
			Ranges: []SweepRange{steppedRange("a", realOf(math.NaN()), realOf(4), realOf(1))}}},
		{"end is not a number", SweepPlan{
			Ranges: []SweepRange{steppedRange("a", realOf(0), realOf(math.NaN()), realOf(1))}}},
		{"step is not a number", SweepPlan{
			Ranges: []SweepRange{steppedRange("a", realOf(0), realOf(4), realOf(math.NaN()))}}},
		{"start is infinite", SweepPlan{
			Ranges: []SweepRange{steppedRange("a", realOf(math.Inf(-1)), realOf(4), realOf(1))}}},
		{"end is infinite", SweepPlan{
			Ranges: []SweepRange{steppedRange("a", realOf(0), realOf(math.Inf(1)), realOf(1))}}},
		{"step is infinite", SweepPlan{
			Ranges: []SweepRange{steppedRange("a", realOf(0), realOf(4), realOf(math.Inf(1)))}}},
		{"a drawn range is infinite", SweepPlan{
			Ranges:  []SweepRange{rangeOf("a", realOf(0), realOf(math.Inf(1)))},
			Sampled: true, Samples: 3, Seed: 7}},
		{"an endpoint carrying a unit is infinite", SweepPlan{
			Ranges: []SweepRange{steppedRange("a", quantity(0), quantity(math.Inf(1)), quantity(1))}}},
		{"a converted endpoint is not a number", SweepPlan{
			Ranges: []SweepRange{steppedRange("a", quantity(math.NaN()), kilometre, quantity(1))}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := refuseSweep(t, ctx, scope, "Ratio", tc.plan)
			if !errors.Is(err, ErrSweepRange) || !strings.Contains(err.Error(), "finite") {
				t.Errorf("err = %v; want an ErrSweepRange naming a non-finite number", err)
			}
		})
	}
}

// A range whose end lies just short of the next step stops at the end: the
// rounding error a count of steps is allowed never buys a whole extra run.
func TestSweepRealRangeStopsAtItsEnd(t *testing.T) {
	ctx, scope := sweepFixture(t)
	table := runSweepOver(t, ctx, scope, "Ratio", SweepPlan{
		Ranges: []SweepRange{
			steppedRange("a", realOf(0), realOf(2e9-0.5), realOf(1e9)),
			steppedRange("b", realOf(1), realOf(1), realOf(1)),
		},
	})
	want := []float64{0, 1e9}
	if len(table.Rows) != len(want) {
		t.Fatalf("the range took %d run(s); want %d, the end being short of the next step",
			len(table.Rows), len(want))
	}
	for i, value := range want {
		if got := table.Rows[i].Bindings[0].Value.Const.Real; got != value {
			t.Errorf("row %d ran a = %v; want %v", i, got, value)
		}
	}
}

// A range as wide as the reals reach draws values inside it: the width of the
// range overflows, which no drawn value may.
func TestSamplesOverTheWidestRealRangeStayInIt(t *testing.T) {
	ctx, scope := sweepFixture(t)
	table := runSweepOver(t, ctx, scope, "Ratio", SweepPlan{
		Ranges: []SweepRange{
			rangeOf("a", realOf(-math.MaxFloat64), realOf(math.MaxFloat64)),
			rangeOf("b", realOf(1), realOf(1)),
		},
		Sampled: true, Samples: 32, Seed: 5,
	})
	for i := range table.Rows {
		drawn := table.Rows[i].Bindings[0].Value.Const.Real
		if math.IsNaN(drawn) || math.IsInf(drawn, 0) {
			t.Fatalf("row %d drew %v; want a finite value", i, drawn)
		}
		if drawn < -math.MaxFloat64 || drawn >= math.MaxFloat64 {
			t.Errorf("row %d drew %v; want a value in [-MaxFloat64, MaxFloat64)", i, drawn)
		}
	}
}

// A range asking for more runs than any budget allows is refused promptly: how
// many runs it takes is counted, not enumerated.
func TestSweepRefusesWideRealRangesPromptly(t *testing.T) {
	ctx, scope := sweepFixture(t)
	sym := calcNamed(t, scope, "Ratio")
	for _, r := range []SweepRange{
		steppedRange("a", realOf(0), realOf(1e18), realOf(1)),
		steppedRange("a", realOf(-math.MaxFloat64), realOf(math.MaxFloat64), realOf(1e-3)),
	} {
		plan := SweepPlan{Ranges: []SweepRange{r, steppedRange("b", realOf(1), realOf(1), realOf(1))}}
		done := make(chan error, 1)
		go func() {
			_, err := ctx.RunSweep(context.Background(), "test::Ratio", plan, sweepCalcRun(ctx, sym, scope))
			done <- err
		}()
		select {
		case err := <-done:
			if !errors.Is(err, ErrSweepBudget) {
				t.Errorf("range ending at %s: got %v; want a budget refusal", FormatValue(r.To), err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("range ending at %s was still being counted after ten seconds", FormatValue(r.To))
		}
	}
}

// A range whose end is as large as the reals reach still runs: allowing its end
// a rounding error must not push the end itself out of range.
func TestSweepRunsToTheLargestRealEndpoint(t *testing.T) {
	ctx, scope := sweepFixture(t)
	table := runSweepOver(t, ctx, scope, "Ratio", SweepPlan{
		Ranges: []SweepRange{
			steppedRange("a", realOf(0), realOf(math.MaxFloat64), realOf(math.MaxFloat64/2)),
			steppedRange("b", realOf(1), realOf(1), realOf(1)),
		},
	})
	want := []float64{0, math.MaxFloat64 / 2, math.MaxFloat64}
	if len(table.Rows) != len(want) {
		t.Fatalf("the range took %d run(s); want %d", len(table.Rows), len(want))
	}
	for i, value := range want {
		if got := table.Rows[i].Bindings[0].Value.Const.Real; got != value {
			t.Errorf("row %d ran a = %v; want %v", i, got, value)
		}
	}
}
