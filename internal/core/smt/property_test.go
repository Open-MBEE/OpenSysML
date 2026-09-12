package smt

import (
	"context"
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// solveStatus answers one query, failing the test on a solver error.
func solveStatus(t *testing.T, solver *solve.Solver, q *solve.Query) solve.Status {
	t.Helper()
	result, err := solver.Solve(context.Background(), q)
	if err != nil {
		t.Fatalf("solve: %v\n%s", err, solve.Script(q))
	}
	return result.Status
}

// loweredWithIndex lowers the named action and keeps the index its context is over.
func loweredWithIndex(t *testing.T, path, src, fqn string) (*runtime.Context, *symbols.Index, *symbols.Symbol, *lower.ActionGraph) {
	t.Helper()
	ctx, idx := fixture(t, path, src)
	action := lookup(t, idx, fqn)
	graph, err := lower.ToActionGraph(action.Decl, action.Scope)
	if err != nil {
		t.Fatalf("lower %s: %v", fqn, err)
	}
	lower.StartFlow(graph)
	return ctx, idx, action, graph
}

// lookup is the one symbol fqn names.
func lookup(t *testing.T, idx *symbols.Index, fqn string) *symbols.Symbol {
	t.Helper()
	matches := idx.LookupQualified(fqn)
	if len(matches) != 1 {
		t.Fatalf("%s matched %d symbols, want 1", fqn, len(matches))
	}
	return matches[0]
}

const conditionsSrc = `package test {
	private import ScalarValues::*;
	action def A {
		attribute x : Integer = 1;
		requirement positive { require x > 0; }
		constraint small { x < 10 }
		constraint bounded { x > -5 }
		first start;
		action drop { assign x := x - 2; }
		action raise { assign x := x + 20; }
		done;
		succession first start then drop;
		succession first drop then raise;
		succession first raise then done;
	}
}`

// TestConditionPropertiesFollowTheRun: a straight-line action's conditions are violated exactly
// at the states the interpreter reports, and one no state violates is proved.
func TestConditionPropertiesFollowTheRun(t *testing.T) {
	solver := requireSolver(t)
	ctx, idx, action, graph := loweredWithIndex(t, "conditions.sysml", conditionsSrc, "test::A")
	enc, err := Encode(ctx, action, graph, runtime.Held{}, 4, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	cases := []struct {
		name     string
		violated []int // states violating the property
	}{
		{"test::A::positive", []int{2}},
		{"test::A::small", []int{3, 4}},
		{"test::A::bounded", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, err := enc.Condition(ctx, lookup(t, idx, c.name), nil)
			if err != nil {
				t.Fatalf("property: %v", err)
			}
			for i := 0; i <= enc.Moves; i++ {
				want := solve.StatusUnsat
				for _, v := range c.violated {
					if v == i {
						want = solve.StatusSat
					}
				}
				q := enc.query(solve.And(enc.exact(i), p.Violated[i]), "probe")
				if got := solveStatus(t, solver, q); got != want {
					t.Errorf("violated at state %d: %v, want %v", i, got, want)
				}
			}
			violation := solveStatus(t, solver, enc.Violation(p))
			if (violation == solve.StatusSat) != (len(c.violated) > 0) {
				t.Errorf("violation: %v, want sat=%v", violation, len(c.violated) > 0)
			}
			if got := solveStatus(t, solver, enc.Failure(p)); got != solve.StatusUnsat {
				t.Errorf("failure: %v, want unsat", got)
			}
		})
	}
	if got := solveStatus(t, solver, enc.Uncertainty()); got != solve.StatusUnsat {
		t.Errorf("uncertainty at k=4: %v, want unsat (every run ends in 4 moves)", got)
	}
	short, err := Encode(ctx, action, graph, runtime.Held{}, 2, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if got := solveStatus(t, solver, short.Uncertainty()); got != solve.StatusSat {
		t.Errorf("uncertainty at k=2: %v, want sat (a token is still live)", got)
	}
}

// TestConditionReadingNoValueIsUndefined: a condition over a feature no state
// has given a value is not violated but undecidable, which is a failure.
func TestConditionReadingNoValueIsUndefined(t *testing.T) {
	solver := requireSolver(t)
	src := `package test {
	private import ScalarValues::*;
	action def A {
		attribute x : Integer;
		requirement positive { require x > 0; }
		first start;
		action set { assign x := 3; }
		done;
		succession first start then set;
		succession first set then done;
	}
}`
	ctx, idx, action, graph := loweredWithIndex(t, "undefined.sysml", src, "test::A")
	enc, err := Encode(ctx, action, graph, runtime.Held{}, 3, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	p, err := enc.Condition(ctx, lookup(t, idx, "test::A::positive"), nil)
	if err != nil {
		t.Fatalf("property: %v", err)
	}
	if got := solveStatus(t, solver, enc.Violation(p)); got != solve.StatusUnsat {
		t.Errorf("violation: %v, want unsat", got)
	}
	if got := solveStatus(t, solver, enc.Failure(p)); got != solve.StatusSat {
		t.Errorf("failure: %v, want sat (x has no value at the start)", got)
	}
	if got := solveStatus(t, solver, enc.Failure(nil)); got != solve.StatusUnsat {
		t.Errorf("failure of the run alone: %v, want unsat", got)
	}
}

// TestConditionOverAnObjectIsRefused: a condition reading a feature the action
// does not carry is refused as not encoded, naming the feature.
func TestConditionOverAnObjectIsRefused(t *testing.T) {
	src := `package test {
	private import ScalarValues::*;
	part def P { attribute level : Integer = 1; }
	action def A {
		attribute x : Integer = 1;
		first start;
		done;
		succession first start then done;
	}
	part p : P {
		requirement positive { require level > 0; }
	}
}`
	ctx, idx, action, graph := loweredWithIndex(t, "object.sysml", src, "test::A")
	enc, err := Encode(ctx, action, graph, runtime.Held{}, 2, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	_, err = enc.Condition(ctx, lookup(t, idx, "test::p::positive"), nil)
	var unsupported *UnsupportedError
	if !errors.As(err, &unsupported) || !errors.Is(err, ErrNotEncoded) {
		t.Fatalf("condition over an object: %v, want an UnsupportedError", err)
	}
	t.Log(err)
}

// TestDeadlockProperty: a join one branch never reaches deadlocks on every
// schedule, which the property reports; the fork/join that completes is
// deadlock-free and proved so.
func TestDeadlockProperty(t *testing.T) {
	solver := requireSolver(t)
	stuck := `package test {
	action stuck {
		first start;
		action left;
		action right;
		join sync;
		done;
		succession first start then left;
		succession first left then sync;
		succession first right then sync;
		succession first sync then done;
	}
}`
	ctx, action, graph := loweredAction(t, stuck, "test::stuck")
	enc, err := Encode(ctx, action, graph, runtime.Held{}, 4, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	p := enc.Deadlock()
	if got := solveStatus(t, solver, enc.Violation(p)); got != solve.StatusSat {
		t.Errorf("deadlock: %v, want sat", got)
	}
	if got := solveStatus(t, solver, enc.Failure(p)); got != solve.StatusUnsat {
		t.Errorf("failure: %v, want unsat", got)
	}
	// Once deadlocked, the run is over: no run of the flow is cut by the bound.
	if got := solveStatus(t, solver, enc.Uncertainty()); got != solve.StatusUnsat {
		t.Errorf("uncertainty: %v, want unsat", got)
	}

	free := `package test {
	action free {
		first start;
		fork split;
		action left;
		action right;
		join sync;
		done;
		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then done;
	}
}`
	ctx, action, graph = loweredAction(t, free, "test::free")
	enc, err = Encode(ctx, action, graph, runtime.Held{}, 6, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	p = enc.Deadlock()
	if got := solveStatus(t, solver, enc.Violation(p)); got != solve.StatusUnsat {
		t.Errorf("deadlock of the free flow: %v, want unsat", got)
	}
	if got := solveStatus(t, solver, enc.Uncertainty()); got != solve.StatusUnsat {
		t.Errorf("uncertainty at k=6: %v, want unsat", got)
	}
	short, err := Encode(ctx, action, graph, runtime.Held{}, 5, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if got := solveStatus(t, solver, short.Uncertainty()); got != solve.StatusSat {
		t.Errorf("uncertainty at k=5: %v, want sat", got)
	}
}
