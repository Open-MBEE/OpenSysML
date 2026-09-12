package smt

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// document is one indexed document the engine is asked about: its index and the
// building model a surface would hand the analysis.
type document struct {
	idx   *symbols.Index
	model *analysis.Model
}

// indexed indexes src over the standard library, as the surfaces do.
func indexed(t *testing.T, path, src string) *document {
	t.Helper()
	idx := libs.NewModelIndex()
	sf := source.New(path, []byte(src))
	idx.AddDocument(path, parser.New(sf).ParseFile())
	idx.ExpandWildcardImports()
	model := &analysis.Model{
		Semantics: func() (*runtime.Model, error) {
			resolver := resolve.New(idx)
			m := runtime.NewModel(semantics.NewModel(resolver), resolver)
			m.RegisterSource(sf)
			return m, nil
		},
		Fresh: func(w *analysis.Worker) (*runtime.Context, error) {
			return runtime.NewContext(w.Model, 10000), nil
		},
	}
	return &document{idx: idx, model: model}
}

// holds is the Holds question about the behavior fqn names and the condition
// (none for deadlock freedom alone), the schedule free.
func (d *document) holds(t *testing.T, fqn, condition string) analysis.Question {
	t.Helper()
	behavior := lookup(t, d.idx, fqn)
	ask := &analysis.HoldsAsk{
		Behavior: behavior,
		Start: func(ctx *runtime.Context) (*runtime.ActionExecutor, error) {
			return ctx.CreateActionExecutor(behavior)
		},
	}
	if condition != "" {
		ask.Condition = lookup(t, d.idx, condition)
	}
	return analysis.Question{Kind: analysis.Holds, Subject: fqn, Free: analysis.FreeSchedule, Holds: ask}
}

// engine is the smt engine over the discovered solver, skipping without one.
func engine(t *testing.T) *Engine {
	t.Helper()
	requireSolver(t)
	return New(nil)
}

// answer asks the engine one question under the budget.
func answer(t *testing.T, e *Engine, d *document, q analysis.Question, budget analysis.Budget) analysis.Result {
	t.Helper()
	result, err := e.Run(context.Background(), d.model, q, budget)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return result
}

// expect checks the claim and strength of an answer.
func expect(t *testing.T, result analysis.Result, claim analysis.Claim, strength analysis.Strength) {
	t.Helper()
	if result.Claim != claim || result.Strength != strength {
		t.Fatalf("answer %v/%v (%s), want %v/%v", result.Claim, result.Strength, result.Reason, claim, strength)
	}
}

// bound is the named bound of a result.
func bound(t *testing.T, result analysis.Result, name string) analysis.Bound {
	t.Helper()
	for _, b := range result.Bounds {
		if b.Name == name {
			return b
		}
	}
	t.Fatalf("no bound %q in %v", name, result.Bounds)
	return analysis.Bound{}
}

// TestEngineDescribesItself: the engine is `smt`, answers Holds with the schedule
// free over concrete inputs, replays, and reaches proof.
func TestEngineDescribesItself(t *testing.T) {
	e := New(nil)
	if e.Name() != EngineName {
		t.Fatalf("name %q", e.Name())
	}
	d := e.Describe()
	if len(d.Questions) != 1 || d.Questions[0] != analysis.Holds || !d.Replays || d.Authority != analysis.Proved {
		t.Fatalf("description %+v", d)
	}
	var _ analysis.External = e
}

// TestEngineRefusesWhatItDoesNotAnswer: another kind, free inputs, a fixed schedule
// and a Holds question without its ask are each refused with the typed reason, and a
// solver's absence is the typed absence, from Process and from Covers alike.
func TestEngineRefusesWhatItDoesNotAnswer(t *testing.T) {
	d := indexed(t, "refuse.sysml", conditionsSrc)
	e := New(func() (*solve.Solver, error) { return &solve.Solver{Name: "z3", Path: "/bin/z3"}, nil })
	ok := d.holds(t, "test::A", "")
	cases := []struct {
		name string
		q    analysis.Question
		want error
	}{
		{"kind", analysis.Question{Kind: analysis.Outcomes, Free: analysis.FreeSchedule}, analysis.ErrNotAsked},
		{"inputs", analysis.Question{Kind: analysis.Holds, Free: analysis.FreeSchedule | analysis.FreeInputs, Holds: ok.Holds}, analysis.ErrFreedom},
		{"schedule", analysis.Question{Kind: analysis.Holds, Holds: ok.Holds}, ErrScheduleFixed},
		{"ask", analysis.Question{Kind: analysis.Holds, Free: analysis.FreeSchedule}, analysis.ErrMalformedQuestion},
		{"behavior", analysis.Question{Kind: analysis.Holds, Free: analysis.FreeSchedule, Holds: &analysis.HoldsAsk{Start: ok.Holds.Start}}, analysis.ErrMalformedQuestion},
		{"start", analysis.Question{Kind: analysis.Holds, Free: analysis.FreeSchedule, Holds: &analysis.HoldsAsk{Behavior: ok.Holds.Behavior}}, analysis.ErrMalformedQuestion},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			coverage := e.Covers(d.model, c.q)
			if coverage.Covered || !errors.Is(coverage.Refusal, c.want) {
				t.Fatalf("coverage %+v, want %v", coverage, c.want)
			}
			if _, err := e.Run(context.Background(), d.model, c.q, analysis.Budget{}); !errors.Is(err, c.want) {
				t.Fatalf("run: %v, want %v", err, c.want)
			}
		})
	}
	if coverage := e.Covers(d.model, ok); !coverage.Covered {
		t.Fatalf("a Holds question with the schedule free is refused: %v", coverage.Refusal)
	}
	if name, err := e.Process(); err != nil || name != "z3 at /bin/z3" {
		t.Fatalf("process %q, %v", name, err)
	}
	absent := New(func() (*solve.Solver, error) { return nil, solve.ErrNoSolver })
	var missing *analysis.ProcessAbsentError
	if _, err := absent.Process(); !errors.As(err, &missing) || missing.Engine != EngineName {
		t.Fatalf("process without a solver: %v", err)
	}
	if coverage := absent.Covers(d.model, ok); coverage.Covered || !errors.Is(coverage.Refusal, analysis.ErrProcessAbsent) {
		t.Fatalf("coverage without a solver: %+v", coverage)
	}
}

// TestEngineDecidesConditions: a violated requirement is witnessed and its witness replays; an
// unviolated constraint is proved within the bound and bounded when the bound cuts the run short.
func TestEngineDecidesConditions(t *testing.T) {
	e := engine(t)
	d := indexed(t, "conditions.sysml", conditionsSrc)

	violated := answer(t, e, d, d.holds(t, "test::A", "test::A::positive"), analysis.Budget{Depth: 4})
	expect(t, violated, analysis.ClaimViolated, analysis.Witnessed)
	if violated.Witness == nil {
		t.Fatal("no witness")
	}
	if _, ok := violated.Witness.Schedule.Replay(); !ok {
		t.Fatalf("witness schedule %s is not a replay", violated.Witness.Schedule)
	}
	if len(violated.Witness.Choices) != 0 {
		t.Fatalf("a straight-line run has no choice points, got %v", violated.Witness.Choices)
	}
	if len(violated.Values) != 1 || violated.Values[0].Err == nil {
		t.Fatalf("the interpreter's violation is not reported: %+v", violated.Values)
	}
	var violation *runtime.ViolationError
	if !errors.As(violated.Values[0].Err, &violation) {
		t.Fatalf("the interpreter's report %v is not a violation", violated.Values[0].Err)
	}

	proved := answer(t, e, d, d.holds(t, "test::A", "test::A::bounded"), analysis.Budget{Depth: 4})
	expect(t, proved, analysis.ClaimHolds, analysis.Proved)
	for _, b := range proved.Bounds {
		if b.Reached {
			t.Errorf("a proof reached bound %s", b.Name)
		}
	}
	if moves := bound(t, proved, "moves"); moves.Limit != 4 {
		t.Errorf("moves bound %+v, want the budget's depth", moves)
	}
	if unroll := bound(t, proved, "unroll"); unroll.Limit != DefaultUnroll {
		t.Errorf("unroll bound %+v, want %d", unroll, DefaultUnroll)
	}

	bounded := answer(t, e, d, d.holds(t, "test::A", "test::A::bounded"), analysis.Budget{Depth: 2})
	expect(t, bounded, analysis.ClaimHolds, analysis.Bounded)
	if moves := bound(t, bounded, "moves"); !moves.Reached {
		t.Errorf("moves bound %+v, want reached", moves)
	}
}

// TestEngineDecidesDeadlock: a join one branch never reaches deadlocks on every
// schedule, witnessed and replayed to the interpreter's own deadlock; a flow that
// completes is deadlock-free, proved.
func TestEngineDecidesDeadlock(t *testing.T) {
	e := engine(t)
	src := `package test {
	action def Stuck {
		first start;
		action stranded;
		join j;
		done;
		succession first start then j;
		succession first stranded then j;
		succession first j then done;
	}
	action def Flows {
		first start;
		fork f;
		action a;
		action b;
		join j;
		done;
		succession first start then f;
		succession first f then a;
		succession first f then b;
		succession first a then j;
		succession first b then j;
		succession first j then done;
	}
}`
	d := indexed(t, "deadlock.sysml", src)
	stuck := answer(t, e, d, d.holds(t, "test::Stuck", ""), analysis.Budget{Depth: 6})
	expect(t, stuck, analysis.ClaimViolated, analysis.Witnessed)
	if len(stuck.Values) != 1 || !errors.Is(stuck.Values[0].Err, runtime.ErrActionDeadlock) {
		t.Fatalf("the interpreter's deadlock is not reported: %+v", stuck.Values)
	}
	flows := answer(t, e, d, d.holds(t, "test::Flows", ""), analysis.Budget{Depth: 8})
	expect(t, flows, analysis.ClaimHolds, analysis.Proved)
}

// TestEngineRefusesWhatItDoesNotEncode: a body using a construct outside the
// encoding is not covered, naming the node, before any query is asked.
func TestEngineRefusesWhatItDoesNotEncode(t *testing.T) {
	src := `package test {
	action def Waits {
		first start;
		action wait accept s : Signal;
		done;
		succession first start then wait;
		succession first wait then done;
	}
	item def Signal;
}`
	d := indexed(t, "accept.sysml", src)
	// A solver that cannot run: any query asked would be the run's error.
	e := New(func() (*solve.Solver, error) { return &solve.Solver{Name: "none", Path: "/nonexistent"}, nil })
	result := answer(t, e, d, d.holds(t, "test::Waits", ""), analysis.Budget{Depth: 4})
	expect(t, result, analysis.ClaimNone, analysis.NotCovered)
	if !strings.Contains(result.Reason, ErrNotEncoded.Error()) || !strings.Contains(result.Reason, "wait") {
		t.Fatalf("reason %q does not name the construct and its node", result.Reason)
	}
}

// TestEngineReportsUnknownAsNotCovered: a solver that runs out of the budget's
// time on the first query leaves the question not covered, never proved, with the
// solver bound reached and the undecided query reported.
func TestEngineReportsUnknownAsNotCovered(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skipf("no sh to stand in for a hanging solver: %v", err)
	}
	hanging := &solve.Solver{Name: "hanging", Path: "sh", Args: []string{"-c", "exec sleep 30"},
		Declared: solve.DeclaredCapabilities("hanging", solve.AllCapabilities...)}
	e := New(func() (*solve.Solver, error) { return hanging, nil })
	d := indexed(t, "hang.sysml", conditionsSrc)
	result := answer(t, e, d, d.holds(t, "test::A", "test::A::bounded"), analysis.Budget{Depth: 3, Solver: 100 * time.Millisecond})
	expect(t, result, analysis.ClaimNone, analysis.NotCovered)
	if !strings.Contains(result.Reason, "did not decide") {
		t.Errorf("reason %q", result.Reason)
	}
	if solverBound := bound(t, result, "solver"); !solverBound.Reached || solverBound.Limit != 100 {
		t.Errorf("solver bound %+v, want reached at 100ms", solverBound)
	}
	if len(result.Values) != 1 || result.Values[0].Solved == nil || !result.Values[0].Solved.TimedOut {
		t.Errorf("the undecided query is not reported: %+v", result.Values)
	}
}

// TestEngineReportsUndecidedUncertaintyAsNotCovered: a solver that refutes every
// violation but runs out of time on whether the bounds were reached leaves the
// question not covered — not a bounded holds — with the undecided query reported.
func TestEngineReportsUndecidedUncertaintyAsNotCovered(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skipf("no sh to stand in for a solver: %v", err)
	}
	// Each query is a fresh process; the count file has the fourth one hang.
	count := filepath.Join(t.TempDir(), "asked")
	script := `n=$(cat "$1" 2>/dev/null || echo 0); echo $((n+1)) > "$1"
if [ "$n" -ge 3 ]; then exec sleep 30; fi
while IFS= read -r line; do case "$line" in *"(check-sat)"*) echo unsat;; esac; done`
	unsure := &solve.Solver{Name: "unsure", Path: "sh", Args: []string{"-c", script, "unsure", count},
		Declared: solve.DeclaredCapabilities("unsure", solve.AllCapabilities...)}
	e := New(func() (*solve.Solver, error) { return unsure, nil })
	d := indexed(t, "unsure.sysml", conditionsSrc)
	result := answer(t, e, d, d.holds(t, "test::A", "test::A::bounded"), analysis.Budget{Depth: 3, Solver: 100 * time.Millisecond})
	expect(t, result, analysis.ClaimNone, analysis.NotCovered)
	if !strings.Contains(result.Reason, "did not decide whether every schedule ends within the bounds") {
		t.Errorf("reason %q", result.Reason)
	}
	if solverBound := bound(t, result, "solver"); !solverBound.Reached || solverBound.Limit != 100 {
		t.Errorf("solver bound %+v, want reached at 100ms", solverBound)
	}
	if len(result.Values) != 1 || result.Values[0].Solved == nil || !result.Values[0].Solved.TimedOut {
		t.Errorf("the undecided query is not reported: %+v", result.Values)
	}
	if asked, err := os.ReadFile(count); err != nil || strings.TrimSpace(string(asked)) != "4" {
		t.Errorf("queries asked %q, %v; want the three property queries and the uncertainty query", asked, err)
	}
}

// TestEngineStartsFromTheValuesHeld: the run begins from the values the
// performance holds when started, ahead of the defaults the action declares —
// an input the start supplies decides the answer, and the witness replays with it.
func TestEngineStartsFromTheValuesHeld(t *testing.T) {
	e := engine(t)
	d := indexed(t, "held.sysml", `package test {
	private import ScalarValues::*;
	action def A {
		in x : Integer = 1;
		constraint small { x < 10 }
		first start;
		action raise { assign x := x + 5; }
		done;
		succession first start then raise;
		succession first raise then done;
	}
}`)
	byDefault := answer(t, e, d, d.holds(t, "test::A", "test::A::small"), analysis.Budget{Depth: 4})
	expect(t, byDefault, analysis.ClaimHolds, analysis.Proved)

	behavior := lookup(t, d.idx, "test::A")
	twenty := runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 20}}
	supplied := analysis.Question{Kind: analysis.Holds, Subject: "test::A", Free: analysis.FreeSchedule, Holds: &analysis.HoldsAsk{
		Behavior:  behavior,
		Condition: lookup(t, d.idx, "test::A::small"),
		Start: func(ctx *runtime.Context) (*runtime.ActionExecutor, error) {
			return ctx.CreateActionExecutorWithInputs(behavior, nil, map[string]runtime.Value{"x": twenty})
		},
	}}
	violated := answer(t, e, d, supplied, analysis.Budget{Depth: 4})
	expect(t, violated, analysis.ClaimViolated, analysis.Witnessed)
	if violated.Witness == nil {
		t.Fatal("no witness")
	}
	var violation *runtime.ViolationError
	if len(violated.Values) != 1 || !errors.As(violated.Values[0].Err, &violation) {
		t.Fatalf("the interpreter's violation is not reported: %+v", violated.Values)
	}
}

// TestEngineDoesNotProveOverRoundedArithmetic: a condition over real arithmetic
// the interpreter rounds in float64 is not proved by an exact-real unsat; the
// answer is not covered, naming the rounding. A violation the same arithmetic
// reaches is still witnessed, once the replay confirms it in float64.
func TestEngineDoesNotProveOverRoundedArithmetic(t *testing.T) {
	e := engine(t)
	d := indexed(t, "rounded.sysml", `package test {
	private import ScalarValues::*;
	action def R {
		attribute x : Real = 0.5;
		constraint small { x < 10.0 }
		constraint tiny { x < 0.75 }
		first start;
		action raise { assign x := x + 0.25; }
		done;
		succession first start then raise;
		succession first raise then done;
	}
}`)
	result := answer(t, e, d, d.holds(t, "test::R", "test::R::small"), analysis.Budget{Depth: 4})
	expect(t, result, analysis.ClaimNone, analysis.NotCovered)
	if !strings.Contains(result.Reason, "rounds in floating point") {
		t.Errorf("reason %q", result.Reason)
	}
	if len(result.Values) != 1 || result.Values[0].Solved == nil || result.Values[0].Solved.Status != solve.StatusUnsat {
		t.Errorf("the undeciding unsat is not reported: %+v", result.Values)
	}

	violated := answer(t, e, d, d.holds(t, "test::R", "test::R::tiny"), analysis.Budget{Depth: 4})
	expect(t, violated, analysis.ClaimViolated, analysis.Witnessed)
	var violation *runtime.ViolationError
	if len(violated.Values) != 1 || !errors.As(violated.Values[0].Err, &violation) {
		t.Fatalf("the interpreter's violation is not reported: %+v", violated.Values)
	}
}

// TestEngineEncodesInheritedFeatures: a specialized action's inherited features
// are encoded as the performance holds them, with the inherited default or the
// redefinition's, so a condition over one is decided as the interpreter decides it.
func TestEngineEncodesInheritedFeatures(t *testing.T) {
	e := engine(t)
	d := indexed(t, "inherited.sysml", `package test {
	private import ScalarValues::*;
	action def Base {
		attribute x : Integer = 1;
		attribute y : Integer = 1;
	}
	action def Derived :> Base {
		attribute :>> y = 20;
		constraint small { x < 10 }
		constraint bounded { y < 10 }
		first start;
		action raise { assign x := x + 1; }
		done;
		succession first start then raise;
		succession first raise then done;
	}
}`)
	inherited := answer(t, e, d, d.holds(t, "test::Derived", "test::Derived::small"), analysis.Budget{Depth: 4})
	expect(t, inherited, analysis.ClaimHolds, analysis.Proved)

	redefined := answer(t, e, d, d.holds(t, "test::Derived", "test::Derived::bounded"), analysis.Budget{Depth: 4})
	expect(t, redefined, analysis.ClaimViolated, analysis.Witnessed)
	var violation *runtime.ViolationError
	if len(redefined.Values) != 1 || !errors.As(redefined.Values[0].Err, &violation) {
		t.Fatalf("the interpreter's violation is not reported: %+v", redefined.Values)
	}
}

// TestEngineStopsConditionsAtTheFirstFailure: a required condition failing
// stops the interpreter before a later one that would divide by zero, so the
// violation is witnessed and replays to the interpreter's own; with the order
// reversed the division is the error the interpreter reports.
func TestEngineStopsConditionsAtTheFirstFailure(t *testing.T) {
	e := engine(t)
	d := indexed(t, "ordered.sysml", `package test {
	private import ScalarValues::*;
	action def Ordered {
		attribute x : Integer = 1;
		requirement guarded { require x > 0; require 10 / x > 1; }
		requirement exposed { require 10 / x > 1; require x > 0; }
		first start;
		action zero { assign x := 0; }
		done;
		succession first start then zero;
		succession first zero then done;
	}
}`)
	guarded := answer(t, e, d, d.holds(t, "test::Ordered", "test::Ordered::guarded"), analysis.Budget{Depth: 4})
	expect(t, guarded, analysis.ClaimViolated, analysis.Witnessed)
	var violation *runtime.ViolationError
	if len(guarded.Values) != 1 || !errors.As(guarded.Values[0].Err, &violation) {
		t.Fatalf("the interpreter's violation is not reported: %+v", guarded.Values)
	}
	exposed := answer(t, e, d, d.holds(t, "test::Ordered", "test::Ordered::exposed"), analysis.Budget{Depth: 4})
	expect(t, exposed, analysis.ClaimViolated, analysis.Witnessed)
	if len(exposed.Values) != 1 || exposed.Values[0].Err == nil || errors.As(exposed.Values[0].Err, &violation) ||
		!strings.Contains(exposed.Values[0].Err.Error(), "zero") {
		t.Fatalf("the interpreter's division by zero is not reported: %+v", exposed.Values)
	}
}

// TestEngineKeepsEveryTokenOfARevisitedFork: a fork two merge arrivals reach
// performs twice, and its four tokens are all in flight at once as the
// interpreter runs them; a condition only those four violate is witnessed.
func TestEngineKeepsEveryTokenOfARevisitedFork(t *testing.T) {
	e := engine(t)
	d := indexed(t, "crowd.sysml", `package test {
	private import ScalarValues::*;
	action def Crowd {
		attribute live : Integer = 0;
		constraint few { live < 4 }
		first start;
		fork f1; action a; action b; merge m; fork f2;
		action c { assign live := live + 1; }
		action d { assign live := live + 1; }
		action e { assign live := live - 1; }
		action g { assign live := live - 1; }
		merge m2; done;
		succession first start then f1;
		succession first f1 then a;
		succession first f1 then b;
		succession first a then m;
		succession first b then m;
		succession first m then f2;
		succession first f2 then c;
		succession first f2 then d;
		succession first c then e;
		succession first d then g;
		succession first e then m2;
		succession first g then m2;
		succession first m2 then done;
	}
}`)
	result := answer(t, e, d, d.holds(t, "test::Crowd", "test::Crowd::few"), analysis.Budget{Depth: 16})
	expect(t, result, analysis.ClaimViolated, analysis.Witnessed)
	var violation *runtime.ViolationError
	if len(result.Values) != 1 || !errors.As(result.Values[0].Err, &violation) {
		t.Fatalf("the interpreter's violation is not reported: %+v", result.Values)
	}
}

// TestEngineWitnessesIntegerOverflow: Integer arithmetic the interpreter refuses
// as overflowing is a failure the solver witnesses, never a value it proves
// about; arithmetic staying within int64 is proved as before.
func TestEngineWitnessesIntegerOverflow(t *testing.T) {
	e := engine(t)
	d := indexed(t, "overflow.sysml", `package test {
	private import ScalarValues::*;
	action def Sum {
		attribute x : Integer = 9223372036854775807;
		constraint positive { x > 0 }
		first start;
		action step { assign x := x + 1; }
		done;
		succession first start then step;
		succession first step then done;
	}
	action def Difference {
		attribute x : Integer = -9223372036854775807;
		constraint negative { x < 0 }
		first start;
		action step { assign x := x - 2; }
		done;
		succession first start then step;
		succession first step then done;
	}
	action def Product {
		attribute x : Integer = 4294967296;
		constraint positive { x > 0 }
		first start;
		action step { assign x := x * x; }
		done;
		succession first start then step;
		succession first step then done;
	}
	action def Negation {
		attribute x : Integer = -9223372036854775807;
		constraint negative { x < 0 }
		first start;
		action step { assign x := -(x - 1); }
		done;
		succession first start then step;
		succession first step then done;
	}
	action def Within {
		attribute x : Integer = 9223372036854775806;
		constraint positive { x > 0 }
		first start;
		action step { assign x := x + 1; }
		done;
		succession first start then step;
		succession first step then done;
	}
}`)
	for _, c := range []struct{ action, condition string }{
		{"test::Sum", "test::Sum::positive"},
		{"test::Difference", "test::Difference::negative"},
		{"test::Product", "test::Product::positive"},
		{"test::Negation", "test::Negation::negative"},
	} {
		result := answer(t, e, d, d.holds(t, c.action, c.condition), analysis.Budget{Depth: 4})
		expect(t, result, analysis.ClaimViolated, analysis.Witnessed)
		if len(result.Values) != 1 || !errors.Is(result.Values[0].Err, semantics.ErrArithmeticOverflow) {
			t.Errorf("%s: the interpreter's overflow is not reported: %+v", c.action, result.Values)
		}
	}
	within := answer(t, e, d, d.holds(t, "test::Within", "test::Within::positive"), analysis.Budget{Depth: 4})
	expect(t, within, analysis.ClaimHolds, analysis.Proved)
}
