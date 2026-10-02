package smt

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

// repeatedOutcome encodes the action, asserts completion within k moves, and
// reports whether it can complete unfailed, failed, and with feature c at each
// of the listed values.
func repeatedOutcome(t *testing.T, solver *solve.Solver, file, fqn string, k int, candidates []int64) (unfailed, failed bool, values map[int64]bool) {
	t.Helper()
	ctx, action, graph, held := loweredConformanceAction(t, file, fqn)
	enc, err := Encode(ctx, action, graph, held, nil, k, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	last := enc.States[k]
	failedTerm := solve.VarTerm(last.Failed)
	unfailed = status(t, solver, enc, k, solve.Not(failedTerm)) == solve.StatusSat
	failed = status(t, solver, enc, k, failedTerm) == solve.StatusSat
	values = make(map[int64]bool)
	for _, base := range enc.Features {
		if !strings.HasSuffix(base.Name, "::c") {
			continue
		}
		v := last.value(base)
		for _, w := range candidates {
			if status(t, solver, enc, k, solve.And(solve.Not(failedTerm), eq(solve.VarTerm(v), solve.IntTerm(w)))) == solve.StatusSat {
				values[w] = true
			}
		}
	}
	return unfailed, failed, values
}

// TestEncodeRepeatedStepOutcomes: each encoded repeated-step shape completes
// with the values the interpreter reaches for it.
func TestEncodeRepeatedStepOutcomes(t *testing.T) {
	solver := requireSolver(t)
	for _, c := range []struct {
		name, file, fqn string
		k               int
		fails           bool
		values          map[int64]bool
	}{
		{"exact", "action_step_multiplicity_exact.sysml", "test::Rep", 6, false, map[int64]bool{3: true}},
		{"zero", "action_step_multiplicity_zero.sysml", "test::Zero", 6, false, map[int64]bool{7: true}},
		{"fork barrier", "action_step_multiplicity_fork_barrier.sysml", "test::U", 10, false, map[int64]bool{113: true}},
		{"merge fanout", "action_step_multiplicity_merge_fanout.sysml", "test::U", 10, false, map[int64]bool{31: true}},
		{"join per performance", "action_step_multiplicity_join_per_performance.sysml", "test::U", 10, false, map[int64]bool{3: true}},
		{"guard true", "action_step_multiplicity_guard_true.sysml", "test::U", 10, false, map[int64]bool{3: true}},
		{"guard false", "action_step_multiplicity_guard_false.sysml", "test::U", 10, true, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			var candidates []int64
			for w := range c.values {
				candidates = append(candidates, w)
			}
			candidates = append(candidates, 4)
			unfailed, failed, values := repeatedOutcome(t, solver, c.file, c.fqn, c.k, candidates)
			if failed != c.fails {
				t.Errorf("completes failed: %v, want fails=%v", failed, c.fails)
			}
			if unfailed != !c.fails {
				t.Errorf("completes unfailed: %v, want fails=%v", unfailed, c.fails)
			}
			for want := range c.values {
				if !values[want] {
					t.Errorf("c = %d on unfailed completion: unsat, want sat", want)
				}
			}
			for got := range values {
				if !c.values[got] {
					t.Errorf("c = %d on unfailed completion: sat, want unsat", got)
				}
			}
		})
	}
}

// TestEncodeRepeatedStepSharedWriters: two writers racing past a barrier leave
// c at whichever wrote last; the encoding reaches both and none other.
func TestEncodeRepeatedStepSharedWriters(t *testing.T) {
	solver := requireSolver(t)
	const k = 12
	ctx, action, graph, held := loweredDocument(t, "repeated_writers.sysml", `package test {
	private import ScalarValues::*;
	action race {
		attribute c : Integer = 0;
		first start then f;
		fork f;
		then p;
		then b;
		action p;
		action a[2] { assign c := 1; }
		action b { assign c := 2; }
		succession first [1] p then [*] a;
		succession first [*] a then [1] r;
		action r;
		merge m;
		succession first b then m;
		succession first r then m;
		succession first m then done;
	}
}`, "test::race")
	enc, err := Encode(ctx, action, graph, held, nil, k, DefaultUnroll)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	last := enc.States[k]
	failed := solve.VarTerm(last.Failed)
	v := last.Values["test::race::c"]
	if v == nil {
		t.Fatalf("no feature c among %v", names(enc.Features))
	}
	for _, w := range []int64{1, 2} {
		is := eq(solve.VarTerm(v), solve.IntTerm(w))
		if got := status(t, solver, enc, k, solve.And(solve.Not(failed), is)); got != solve.StatusSat {
			t.Errorf("c = %d on unfailed completion: %v, want sat", w, got)
		}
	}
	if got := status(t, solver, enc, k, solve.And(solve.Not(failed), solve.Not(eq(solve.VarTerm(v), solve.IntTerm(1))), solve.Not(eq(solve.VarTerm(v), solve.IntTerm(2))))); got != solve.StatusUnsat {
		t.Errorf("c outside {1,2} on unfailed completion: %v, want unsat", got)
	}
}

// TestAnalyzeRefusesLiveRepeatedStep: a step a token may reach again while its
// performances are live — a cycle, or several successions' arrivals — is
// refused, as is a step whose count is not fixed, and every refusal CheckStep
// declares for the shape: a plain `then`, a control node's contradicting end,
// a guarded succession it does not admit.
func TestAnalyzeRefusesLiveRepeatedStep(t *testing.T) {
	ctx, idx := fixture(t, "<test>", `
		package test {
			private import ScalarValues::*;
			action def Cyclic {
				first start then a;
				action a[2];
				succession first [*] a then [1] b;
				action b;
				succession first [1] b then [*] a;
			}
			action def ManyWays {
				first start then f;
				fork f;
				then p;
				then q;
				action p;
				action q;
				succession first [1] p then [*] a;
				succession first [1] q then [*] a;
				action a[2];
				succession first [*] a then [1] done;
			}
			action def Ranged {
				first start then a;
				action a[0..2];
				then done;
			}
		}`)
	for _, tc := range []struct {
		name, construct, reason string
	}{
		{"Cyclic", "action step multiplicity [2]", "a repeated step a token may reach again while its performances are live is not encoded"},
		{"ManyWays", "action step multiplicity [2]", "a repeated step a token may reach again while its performances are live is not encoded"},
		{"Ranged", "action step multiplicity [0..2]", "the SMT engine requires a fixed single-performance step"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			matches := idx.LookupQualified("test::" + tc.name)
			if len(matches) != 1 {
				t.Fatalf("test::%s matched %d symbols, want one", tc.name, len(matches))
			}
			graph, err := lower.ToActionGraph(matches[0].Decl, matches[0].Scope)
			if err != nil {
				t.Fatalf("lower: %v", err)
			}
			lower.StartFlow(graph)
			_, err = Analyze(graph, ctx.Semantics(), 10)
			var unsupported *UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("Analyze: got %v, want %q", err, tc.reason)
			}
			if unsupported.Construct != tc.construct || unsupported.Reason != tc.reason {
				t.Fatalf("refusal %q/%q, want %q/%q", unsupported.Construct, unsupported.Reason, tc.construct, tc.reason)
			}
		})
	}
}

// TestAnalyzeRefusesWhatCheckStepRefuses: the shapes run and explore refuse —
// a plain `then` around a repeated step, a control node's succession it
// forbids, a guarded one it does not admit — are the engine's refusals too,
// each wrapped so the StepMultiplicityError is still found.
func TestAnalyzeRefusesWhatCheckStepRefuses(t *testing.T) {
	ctx, idx := fixture(t, "<test>", `
		package test {
			private import ScalarValues::*;
			action def PlainThen {
				first start then a;
				action a[2];
				then b;
				action b;
			}
			action def ForkOut {
				first start then f;
				fork f;
				then a;
				then q;
				action a[2];
				action q;
			}
			action def GuardFrom {
				first start then a;
				action a[2];
				action q;
				succession first a if true then q;
			}
		}`)
	for _, tc := range []struct {
		name string
		code string
	}{
		{"PlainThen", lower.StepOrderUnsatisfiableCode},
		{"ForkOut", lower.StepOrderUnsatisfiableCode},
		{"GuardFrom", lower.StepMultiplicityUnsupportedCode},
	} {
		t.Run(tc.name, func(t *testing.T) {
			matches := idx.LookupQualified("test::" + tc.name)
			if len(matches) != 1 {
				t.Fatalf("test::%s matched %d symbols, want one", tc.name, len(matches))
			}
			graph, err := lower.ToActionGraph(matches[0].Decl, matches[0].Scope)
			if err != nil {
				t.Fatalf("lower: %v", err)
			}
			lower.StartFlow(graph)
			_, err = Analyze(graph, ctx.Semantics(), 10)
			var unsupported *UnsupportedError
			var stepErr *lower.StepMultiplicityError
			if !errors.As(err, &unsupported) || !errors.As(err, &stepErr) {
				t.Fatalf("Analyze: got %v, want an UnsupportedError wrapping a StepMultiplicityError", err)
			}
			if stepErr.Code != tc.code {
				t.Errorf("code: got %s, want %s", stepErr.Code, tc.code)
			}
			if unsupported.Construct != "action step multiplicity [2]" {
				t.Errorf("construct: got %q, want the multiplicity of a", unsupported.Construct)
			}
		})
	}
}
