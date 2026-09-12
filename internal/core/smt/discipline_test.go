package smt

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
)

// The verdict discipline: what a bound cuts is bounded, never proved; what the
// interpreter reaches is reported as it reaches it; and the executor's outcomes
// over merges are the encoding's, pinned as referee cases.

const loopsSrc = `package test {
	private import ScalarValues::*;

	// A merge-loop: n rounds of merge, increment, decide.
	action def Rounds {
		attribute i : Integer = 0;
		attribute merged : Integer = 0;
		first start;
		merge again;
		action count { assign merged := merged + 1; assign i := i + 1; }
		decide more;
		done;
		succession first start then again;
		succession first again then count;
		succession first count then more;
		succession first more if i < 3 then again;
		succession first more if i >= 3 then done;
	}

	// A body while: three iterations in one node.
	action def Body {
		attribute n : Integer = 0;
		first start;
		action count { while n < 3 { assign n := n + 1; } }
		done;
		succession first start then count;
		succession first count then done;
	}

	// A fork whose two branches both arrive at one merge.
	action def Meet {
		attribute merged : Integer = 0;
		attribute left : Boolean = false;
		attribute right : Boolean = false;
		first start;
		fork split;
		action a { assign left := true; }
		action b { assign right := true; }
		merge m;
		action count { assign merged := merged + 1; }
		done;
		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then m;
		succession first b then m;
		succession first m then count;
		succession first count then done;
	}

	// One branch divides by a feature the other branch zeroes; the flow is
	// longer than the bound either way.
	action def Divides {
		attribute d : Integer = 5;
		attribute r : Integer = 0;
		first start;
		fork split;
		action a1; action a2; action a3; action a4; action a5; action a6; action a7; action a8;
		action div { assign r := 10 / d; }
		action zero { assign d := 0; }
		action b1; action b2; action b3; action b4; action b5; action b6; action b7; action b8;
		join sync;
		done;
		succession first start then split;
		succession first split then a1;
		succession first a1 then a2;
		succession first a2 then a3;
		succession first a3 then a4;
		succession first a4 then a5;
		succession first a5 then a6;
		succession first a6 then a7;
		succession first a7 then a8;
		succession first a8 then div;
		succession first div then sync;
		succession first split then zero;
		succession first zero then b1;
		succession first b1 then b2;
		succession first b2 then b3;
		succession first b3 then b4;
		succession first b4 then b5;
		succession first b5 then b6;
		succession first b6 then b7;
		succession first b7 then b8;
		succession first b8 then sync;
		succession first sync then done;
	}
}`

// TestMergeLoopBeyondTheBoundIsBounded: a merge-loop of more rounds than the
// moves allow reports no violation within the bound, not a proof; with moves
// enough for every round it is proved.
func TestMergeLoopBeyondTheBoundIsBounded(t *testing.T) {
	e := engine(t)
	d := indexed(t, "loops.sysml", loopsSrc)
	q := d.holds(t, "test::Rounds", "")

	cut := answer(t, e, d, q, analysis.Budget{Depth: 6})
	expect(t, cut, analysis.ClaimHolds, analysis.Bounded)
	if moves := bound(t, cut, "moves"); !moves.Reached {
		t.Errorf("moves bound %+v, want reached", moves)
	}

	whole := answer(t, e, d, q, analysis.Budget{Depth: 16})
	expect(t, whole, analysis.ClaimHolds, analysis.Proved)
}

// TestBodyWhileBeyondUnrollingIsBounded: a body `while` of more iterations than
// the unrolling reports no violation within the bound, the unroll bound reached;
// unrolled far enough it is proved.
func TestBodyWhileBeyondUnrollingIsBounded(t *testing.T) {
	requireSolver(t)
	d := indexed(t, "loops.sysml", loopsSrc)
	q := d.holds(t, "test::Body", "")

	cut := answer(t, New(nil).Unrolling(1), d, q, analysis.Budget{Depth: 6})
	expect(t, cut, analysis.ClaimHolds, analysis.Bounded)
	if unroll := bound(t, cut, "unroll"); !unroll.Reached || unroll.Limit != 1 {
		t.Errorf("unroll bound %+v, want 1 reached", unroll)
	}
	if moves := bound(t, cut, "moves"); moves.Reached {
		t.Errorf("moves bound %+v, want not reached", moves)
	}

	whole := answer(t, New(nil), d, q, analysis.Budget{Depth: 6})
	expect(t, whole, analysis.ClaimHolds, analysis.Proved)
}

// TestDivisionByZeroIsReportedNotBounded: one schedule divides by zero at move
// 12, the bound, while another is still live there; the error is the answer.
func TestDivisionByZeroIsReportedNotBounded(t *testing.T) {
	e := engine(t)
	d := indexed(t, "loops.sysml", loopsSrc)
	result := answer(t, e, d, d.holds(t, "test::Divides", ""), analysis.Budget{Depth: 12})
	expect(t, result, analysis.ClaimViolated, analysis.Witnessed)
	if len(result.Values) != 1 || result.Values[0].Err == nil {
		t.Fatalf("the interpreter's error is not reported: %+v", result.Values)
	}
	if err := result.Values[0].Err; errors.Is(err, runtime.ErrActionDeadlock) || !strings.Contains(err.Error(), "zero") {
		t.Errorf("reported %v, want the division by zero", err)
	}
	if !strings.HasPrefix(result.Reason, "at step 12:") {
		t.Errorf("reason %q, want the error at step 12", result.Reason)
	}
}

// TestPinnedMergeOutcomes: a fork whose branches both reach one merge performs
// what follows twice, and a merge-loop of three rounds three times — the
// executor's outcomes, which the encoding enumerates and replays.
func TestPinnedMergeOutcomes(t *testing.T) {
	solver := requireSolver(t)
	d := indexed(t, "loops.sysml", loopsSrc)
	budget := analysis.Budget{Depth: 16}
	pinned := []struct {
		action   string
		outcomes []string
	}{
		{"test::Meet", []string{"left = true; merged = 2; right = true"}},
		{"test::Rounds", []string{"i = 3; merged = 3"}},
	}
	for _, p := range pinned {
		t.Run(p.action, func(t *testing.T) {
			action := lookup(t, d.idx, p.action)
			encoding, refusal := encodeDocument(t, d, action, budget)
			if refusal != nil {
				t.Fatalf("refused: %v", refusal)
			}
			exploration := explore(t, runtime.DefaultExploreBudget, d.fresh(budget), action)
			if !exploration.Complete() {
				t.Fatalf("exploration %s", exploration.Status())
			}
			outcomes := compareOutcomes(t, solver, encoding, d, action, budget, exploration)
			if !slices.Equal(outcomes.explored, p.outcomes) {
				t.Errorf("the executor reaches %q, the pin says %q", outcomes.explored, p.outcomes)
			}
			if !outcomes.agreeing || outcomes.replayed != outcomes.witnesses || outcomes.witnesses != len(p.outcomes) {
				t.Errorf("%d witnesses, %d replayed, agreeing %v", outcomes.witnesses, outcomes.replayed, outcomes.agreeing)
			}
			if !refereeVerdict(t, solver, d, action, budget, exploration) {
				t.Error("the verdict disagrees with the exploration")
			}
		})
	}
}

// TestEngineWithoutASolverIsAbsent: with no solver on the path the engine's
// process is absent, a typed error and never an answer.
func TestEngineWithoutASolverIsAbsent(t *testing.T) {
	e := New(func() (*solve.Solver, error) { return nil, solve.ErrNoSolver })
	if _, err := e.Process(); !errors.Is(err, solve.ErrNoSolver) {
		t.Fatalf("Process: %v", err)
	}
	d := indexed(t, "loops.sysml", loopsSrc)
	_, err := e.Run(t.Context(), d.model, d.holds(t, "test::Body", ""), analysis.Budget{Depth: 4})
	var absent *analysis.ProcessAbsentError
	if !errors.As(err, &absent) || absent.Engine != EngineName {
		t.Fatalf("Run: %v, want the process absent", err)
	}
}

// TestEncodingEmitsOnlyPortableFeatures: every SMT-LIB feature the relation and
// its queries need — datatypes for nodes and choices, models, the incremental
// enumeration over named variables, the arithmetic of the bodies — is one the
// solve layer's portability harness exercises, so a backend refusing one is
// reported as refusing it.
func TestEncodingEmitsOnlyPortableFeatures(t *testing.T) {
	d := indexed(t, "loops.sysml", loopsSrc)
	budget := analysis.Budget{Depth: 8}
	portable := []solve.Capability{solve.CapModels, solve.CapIncremental, solve.CapDatatypes, solve.CapNonStandardLogic,
		solve.CapIntegerDivision, solve.CapMixedArith, solve.CapNonlinearArith, solve.CapStrings}
	for _, name := range []string{"test::Rounds", "test::Meet", "test::Divides"} {
		encoding, refusal := encodeDocument(t, d, lookup(t, d.idx, name), budget)
		if refusal != nil {
			t.Fatalf("%s refused: %v", name, refusal)
		}
		deadlock := encoding.Deadlock()
		for _, q := range []*solve.Query{encoding.Violation(deadlock), encoding.Failure(deadlock), encoding.Uncertainty(), encoding.Completion()} {
			for _, c := range q.Requires() {
				if !slices.Contains(portable, c) {
					t.Errorf("%s: query %s needs %s, which the portability harness does not exercise", name, q.Kind, c)
				}
			}
		}
	}
}
