package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// evalUnderElementBudget evaluates expr with room for maxElements materialized
// elements, and enough steps that the element ceiling is what stops it.
func evalUnderElementBudget(t *testing.T, expr string, maxElements int64) (Value, error) {
	t.Helper()
	ec, value := collectionExprContext(t, expr, DefaultMaxSteps)
	ec.ctx.maxElements = maxElements
	return ec.Eval(value)
}

// TestElementBudgetBoundsEveryMaterialization requires each way of materializing
// a sequence to be charged: what costs memory is the element, not the operation
// that produced it.
func TestElementBudgetBoundsEveryMaterialization(t *testing.T) {
	for _, expr := range []string{
		"1..100",
		"(1, 2, 3, 4, 5)",
		"xs->collect{in i; i * i}",
		"xs->including(4)",
		"xs->union(ys)",
		"xs->select{in i; i > 0}",
		"xs->subsequence(1, 3)",
		"xs->tail()",
	} {
		if _, err := evalUnderElementBudget(t, expr, 2); err == nil {
			t.Errorf("%s: want the element budget's error, got a value", expr)
		} else if !errors.Is(err, ErrElementLimitExceeded) {
			t.Errorf("%s: error = %v, want ErrElementLimitExceeded", expr, err)
		}
		if _, err := evalUnderElementBudget(t, expr, DefaultMaxElements); err != nil {
			t.Errorf("%s: rejected under the default budget: %v", expr, err)
		}
	}
}

// TestElementBudgetIsNotTheStepBudget requires a range too large to hold to
// report the element ceiling rather than the step one: they bound different
// things, so the remedy the message names has to be the right variable.
func TestElementBudgetIsNotTheStepBudget(t *testing.T) {
	_, err := evalUnderElementBudget(t, "1..1000000", 1000)
	if err == nil {
		t.Fatal("want the element budget's error, got a value")
	}
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Fatalf("error = %v, want ErrElementLimitExceeded", err)
	}
	if errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("error %q also reads as the step budget's", err)
	}
	if !strings.Contains(err.Error(), "(1000 elements") ||
		!strings.Contains(err.Error(), MaxElementsEnvVar) {
		t.Errorf("error %q does not report the budget in force and the variable raising it", err)
	}
}

// TestElementBudgetCountsElementsHeldNotProduced requires a loop building a small
// collection each iteration to run: what the budget bounds is the memory a run
// holds, and an iteration's collection is gone by the next one.
func TestElementBudgetCountsElementsHeldNotProduced(t *testing.T) {
	src := `
		package test {
			calc def Repeat {
				attribute i : Integer = 0;
				attribute acc : Integer = 0;
				while i < 50 {
					assign acc := acc + (1..10)->NumericalFunctions::sum();
					assign i := i + 1;
				}
				acc
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	// Room for two iterations' worth of elements, spent 50 times over.
	ctx.maxElements = 20
	sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", "Repeat")

	result, err := ctx.InvokeCalc(sym, nil, scope)
	if err != nil {
		t.Fatalf("InvokeCalc: %v", err)
	}
	if result.Const.Int != 2750 {
		t.Errorf("acc = %d, want 2750", result.Const.Int)
	}
}

// TestElementBudgetIsReleasedByEveryStep requires a machine whose guard
// materializes a collection on every event to run: a step's collection is gone by
// the next step, so a long run is bounded by what a step holds, not by its length.
func TestElementBudgetIsReleasedByEveryStep(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;

			state machine {
				attribute i : Integer = 0;

				entry; then start;
				state start;
				state step {
					entry { assign i := i + 1; }
				}
				state again {
					entry { assign i := i + 1; }
				}

				succession first start then step;

				transition first step if (1, 2, 3)->NumericalFunctions::sum() > i then again;
				transition first step if (1, 2, 3)->NumericalFunctions::sum() <= i then done;
				transition first again then step;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	// Room for two guard evaluations' worth of elements, spent on every step.
	ctx.maxElements = 6
	sym := findBehavioralSymbol(t, idx.DocumentRoot("<test>"), ast.DefState, ast.UsageState)

	exec, err := newStateExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("create state executor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := exec.stateData["i"].Const.Int; got != 7 {
		t.Errorf("i = %d, want 7", got)
	}
}

// TestElementBudgetIsPerRun requires the count to start over with each run, so a
// session evaluating many collections is not stopped by the ones before.
func TestElementBudgetIsPerRun(t *testing.T) {
	model, resolver, _ := parseAndBuildModel(t, `part def Simple {}`)
	ctx := NewContext(NewModel(model, resolver), DefaultMaxSteps)
	ctx.maxElements = 4

	for i := 0; i < 3; i++ {
		end := ctx.beginRun()
		if err := ctx.chargeElements(4); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		end()
	}
}

// TestElementBudgetBoundsExtents requires each kind of extent — literals, variants and
// objects — to be charged like any other collection.
func TestElementBudgetBoundsExtents(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			enum def Color { red; green; blue; }
			part def Engine;
			part def Wheel;
			variation part def EngineChoice :> Engine {
				variant part four : Engine;
				variant part six : Engine;
				variant part eight : Engine;
			}
			part front : Wheel;
			part rear : Wheel;
			part spare : Wheel;
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	// The wheels are materialized ahead of the budgeted runs, so what a run charges is
	// only what holding the extent costs.
	if _, err := evalIn(t, ctx, pkg.Scope, "(front, rear, spare)"); err != nil {
		t.Fatalf("wheels: %v", err)
	}
	for _, expr := range []string{"all Color", "all EngineChoice", "all Wheel"} {
		ctx.maxElements = 2
		if _, err := evalIn(t, ctx, pkg.Scope, expr); !errors.Is(err, ErrElementLimitExceeded) {
			t.Errorf("%s under a budget of 2: error = %v, want ErrElementLimitExceeded", expr, err)
		}
		ctx.maxElements = DefaultMaxElements
		if _, err := evalIn(t, ctx, pkg.Scope, expr); err != nil {
			t.Errorf("%s under the default budget: %v", expr, err)
		}
	}
}

// TestElementBudgetBoundsStructuredValueReads requires the sequences read off an
// Array, a vector and a vector quantity — their own features and the elements a
// CollectionFunctions operation views — to be charged like any other collection.
func TestElementBudgetBoundsStructuredValueReads(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			private import Collections::*;
			private import VectorFunctions::*;
			private import SI::*;
			attribute grid : Array {
				:>> dimensions = (2, 3);
				:>> elements = (1, 2, 3, 4, 5, 6);
			}
			attribute v = VectorOf((1, 2, 3));
			attribute vq = VectorOf((1, 2, 3)) [m];
		}
	`))
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	// The Array object is materialized ahead of the budgeted runs, so what a run
	// charges is only what reading the value costs. A vector is built by its run:
	// v costs 6 (the literal and the vector), vq costs 9 (and the quantity);
	// cartesianZeroVector costs 9: three vectors of 1, 2 and 3 axes, grouped.
	if _, err := evalIn(t, ctx, pkg.Scope, "grid.rank"); err != nil {
		t.Fatalf("grid.rank: %v", err)
	}
	for _, tc := range []struct {
		expr string
		fits int64 // the least budget the expression evaluates under
	}{
		{"grid.rank", 0},
		{"grid.flattenedSize", 0},
		{"grid.dimensions", 2},
		{"grid.elements", 6},
		{"CollectionFunctions::size(grid)", 6},
		{"CollectionFunctions::'array#'(grid, (2, 3))", 2},
		{"v.dimension", 6},
		{"v.elements", 9},
		{"CollectionFunctions::size(v)", 9},
		{"vq.dimension", 9},
		{"vq.num", 12},
		{"CollectionFunctions::head(vq)", 12},
		{"cartesianZeroVector", 9},
		{"cartesianZeroVector#(3)", 9},
	} {
		if tc.fits > 0 {
			ctx.maxElements = tc.fits - 1
			if _, err := evalIn(t, ctx, pkg.Scope, tc.expr); !errors.Is(err, ErrElementLimitExceeded) {
				t.Errorf("%s under a budget of %d: error = %v, want ErrElementLimitExceeded", tc.expr, tc.fits-1, err)
			}
		}
		ctx.maxElements = tc.fits
		if _, err := evalIn(t, ctx, pkg.Scope, tc.expr); err != nil {
			t.Errorf("%s under a budget of %d: %v", tc.expr, tc.fits, err)
		}
	}
}

// TestElementBudgetIsReleasedByOpenReads requires a model-level read that stops
// short of materializing an open collection to give back what collecting its
// subsetters' values charged: the read holds nothing, so reading it over and
// over costs no more than reading it once.
func TestElementBudgetIsReleasedByOpenReads(t *testing.T) {
	ctx, scope := undeterminedContext(t)
	if _, err := evalIn(t, ctx, scope, "rack.fixed"); err != nil {
		t.Fatalf("rack.fixed: %v", err)
	}
	ctx.maxElements = 1
	reads := strings.Repeat("SequenceFunctions::notEmpty(rack.gear) and ", 4) + "SequenceFunctions::notEmpty(rack.gear)"
	val, err := evalIn(t, ctx, scope, reads)
	wantFormatted(t, reads, val, err, "true")
	val, err = evalIn(t, ctx, scope, "rack.gear")
	wantUndetermined(t, "rack.gear", val, err, "[1..*]")
	if u := val.Undetermined(); u != nil && len(u.Known()) != 1 {
		t.Errorf("rack.gear certainly holds %v, want the one object of fixed", u.Known())
	}
}
