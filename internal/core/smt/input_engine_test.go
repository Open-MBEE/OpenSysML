package smt

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/solve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// freeSrc is inputsSrc without the String attribute, plus the constraints an
// assumption names: one admitting n only where x + n neither fails nor
// overflows, one admitting no n beside it.
const freeSrc = `package test {
	private import ScalarValues::*;
	enum def Mode { Fast; Slow; }
	action def A {
		attribute n : Integer;
		attribute x : Integer = 1;
		attribute u : Natural;
		attribute m : Mode;
		requirement positive { require x + n > 0; }
		requirement natural { require u >= 0; }
		requirement fast { require m == Mode::Fast; }
		constraint nonneg { n >= 0 and n < 100 }
		constraint neg { n < 0 }
		first start;
		action set { assign x := x + 1; }
		done;
		succession first start then set;
		succession first set then done;
	}
}`

// resultInput is the result's input named name.
func resultInput(t *testing.T, result analysis.Result, name string) analysis.Input {
	t.Helper()
	for _, in := range result.Inputs {
		if in.Name == name {
			return in
		}
	}
	t.Fatalf("no input %s among %v", name, result.Inputs)
	return analysis.Input{}
}

// witnessInput is the witness's input line for feature.
func witnessInput(t *testing.T, result analysis.Result, feature string) runtime.InputTaken {
	t.Helper()
	if result.Witness == nil {
		t.Fatal("no witness")
	}
	for _, in := range result.Witness.Inputs {
		if in.Feature == feature {
			return in
		}
	}
	t.Fatalf("witness fixes no %s among %v", feature, result.Witness.Inputs)
	return runtime.InputTaken{}
}

// TestEngineRangesOverUnboundInputs: a requirement holding at the default and
// failing for another value of an unbound input is violated, the witness names
// the value the solver chose and replays through the interpreter's start, the
// answer lists every input with its domain, and its question says the inputs
// were free.
func TestEngineRangesOverUnboundInputs(t *testing.T) {
	e := engine(t)
	d := indexed(t, "free.sysml", freeSrc)
	result := answer(t, e, d, d.holds(t, "test::A", "test::A::positive"), analysis.Budget{Depth: 3})
	expect(t, result, analysis.ClaimViolated, analysis.Witnessed)
	if !result.Question.Free.Has(analysis.FreeInputs) {
		t.Errorf("the answered question does not say the inputs were free: %v", result.Question.Free)
	}
	n := witnessInput(t, result, "n")
	if !strings.HasPrefix(n.Written, "-") {
		t.Errorf("witness n = %q, want a value below -1", n.Written)
	}
	if in := resultInput(t, result, "n"); !in.Free || in.Value != n.Written || in.Type != "Integer" {
		t.Errorf("n listed as %+v, want free Integer at the witness's %s", in, n.Written)
	}
	if in := resultInput(t, result, "x"); in.Free || in.Value != "1" {
		t.Errorf("x listed as %+v, want pinned at 1", in)
	}
	if in := resultInput(t, result, "u"); !in.Free || in.Domain != ">= 0" || in.Type != "Natural" {
		t.Errorf("u listed as %+v, want free Natural in >= 0", in)
	}
	if in := resultInput(t, result, "m"); !in.Free || in.Domain != "{Fast, Slow}" {
		t.Errorf("m listed as %+v, want free in {Fast, Slow}", in)
	}
	w, ok := result.Witness.Schedule.Witness()
	if !ok || len(w.Inputs) != len(result.Witness.Inputs) || w.Inputs[0].Feature != "n" || w.Inputs[0].Written != n.Written {
		t.Errorf("witness schedule %v does not replay the inputs %v", result.Witness.Schedule, result.Witness.Inputs)
	}
}

// TestEngineNarrowsTheDomainToTheDeclaredType: a violation at a negative value
// alone is proved over Natural and violated over Integer, the witness naming the
// negative value.
func TestEngineNarrowsTheDomainToTheDeclaredType(t *testing.T) {
	e := engine(t)
	d := indexed(t, "natural.sysml", freeSrc)
	proved := answer(t, e, d, d.holds(t, "test::A", "test::A::natural"), analysis.Budget{Depth: 3})
	expect(t, proved, analysis.ClaimHolds, analysis.Proved)
	if in := resultInput(t, proved, "u"); !in.Free || in.Domain != ">= 0" || in.Value != "" {
		t.Errorf("u listed as %+v, want free in >= 0 with no value chosen", in)
	}

	integer := indexed(t, "integer.sysml", strings.Replace(freeSrc, "attribute u : Natural;", "attribute u : Integer;", 1))
	violated := answer(t, e, integer, integer.holds(t, "test::A", "test::A::natural"), analysis.Budget{Depth: 3})
	expect(t, violated, analysis.ClaimViolated, analysis.Witnessed)
	if u := witnessInput(t, violated, "u"); !strings.HasPrefix(u.Written, "-") {
		t.Errorf("witness u = %q, want a negative value", u.Written)
	}
	if in := resultInput(t, violated, "u"); !in.Free || in.Domain != "" {
		t.Errorf("u listed as %+v, want free over Integer with no narrowing", in)
	}
}

// TestEngineWitnessNamesAnEnumerationConstructor: an enumeration input's
// witness spells the constructor the solver chose, and the replay reads it.
func TestEngineWitnessNamesAnEnumerationConstructor(t *testing.T) {
	e := engine(t)
	d := indexed(t, "enum.sysml", freeSrc)
	result := answer(t, e, d, d.holds(t, "test::A", "test::A::fast"), analysis.Budget{Depth: 3})
	expect(t, result, analysis.ClaimViolated, analysis.Witnessed)
	if m := witnessInput(t, result, "m"); m.Written != "test::Mode::Slow" {
		t.Errorf("witness m = %q, want test::Mode::Slow", m.Written)
	}
	if in := resultInput(t, result, "m"); in.Value != "test::Mode::Slow" {
		t.Errorf("m listed as %+v, want the witness's constructor", in)
	}
}

// TestEngineAssumesOverTheInitialState: an assumption excluding the violating
// values turns violated into proved and is listed; assumptions admitting no
// initial state decide nothing, and are never proved.
func TestEngineAssumesOverTheInitialState(t *testing.T) {
	e := engine(t)
	d := indexed(t, "assume.sysml", freeSrc)
	q := d.holds(t, "test::A", "test::A::positive")
	q.Holds.Assume = []*symbols.Symbol{lookup(t, d.idx, "test::A::nonneg")}
	result := answer(t, e, d, q, analysis.Budget{Depth: 3})
	expect(t, result, analysis.ClaimHolds, analysis.Proved)
	if len(result.Assumptions) != 1 || result.Assumptions[0] != "constraint nonneg" {
		t.Errorf("assumptions %v, want [constraint nonneg]", result.Assumptions)
	}

	q.Holds.Assume = append(q.Holds.Assume, lookup(t, d.idx, "test::A::neg"))
	result = answer(t, e, d, q, analysis.Budget{Depth: 3})
	expect(t, result, analysis.ClaimNone, analysis.NotCovered)
	if result.Reason != NoInitialState {
		t.Errorf("reason %q, want %q", result.Reason, NoInitialState)
	}
	if len(result.Assumptions) != 2 {
		t.Errorf("assumptions %v, want both listed", result.Assumptions)
	}
}

// TestEngineDoesNotCallRoundedAssumptionsContradictory: assumptions no exact
// initial state satisfies, over arithmetic the interpreter rounds, may still
// admit one in float64, so the answer names the rounding, not a contradiction.
func TestEngineDoesNotCallRoundedAssumptionsContradictory(t *testing.T) {
	e := engine(t)
	d := indexed(t, "rounded_assume.sysml", `package test {
	private import ScalarValues::*;
	action def R {
		attribute x : Real;
		constraint tenth { x == 0.1 }
		constraint inexact { x + 0.2 != 0.3 }
		constraint small { x < 10.0 }
		first start;
		done;
		succession first start then done;
	}
}`)
	q := d.holds(t, "test::R", "test::R::small")
	q.Holds.Assume = []*symbols.Symbol{lookup(t, d.idx, "test::R::tenth"), lookup(t, d.idx, "test::R::inexact")}
	result := answer(t, e, d, q, analysis.Budget{Depth: 3})
	expect(t, result, analysis.ClaimNone, analysis.NotCovered)
	if result.Reason == NoInitialState || !strings.Contains(result.Reason, "assumptions admit an initial state rounds in floating point") {
		t.Errorf("reason %q, want the rounding named", result.Reason)
	}
	if len(result.Values) != 1 || result.Values[0].Solved == nil || result.Values[0].Solved.Status != solve.StatusUnsat {
		t.Errorf("the undeciding unsat is not reported: %+v", result.Values)
	}
	if len(result.Assumptions) != 2 {
		t.Errorf("assumptions %v, want both listed", result.Assumptions)
	}
}

// TestEngineReleasesABoundInput: naming a bound feature drops its binding, so
// the requirement the default satisfied is violated at a value the witness names.
func TestEngineReleasesABoundInput(t *testing.T) {
	e := engine(t)
	d := indexed(t, "release.sysml", strings.Replace(freeSrc, "attribute n : Integer;", "attribute n : Integer = 5;", 1))
	pinned := answer(t, e, d, d.holds(t, "test::A", "test::A::positive"), analysis.Budget{Depth: 3})
	expect(t, pinned, analysis.ClaimHolds, analysis.Proved)
	if in := resultInput(t, pinned, "x"); in.Free || in.Value != "1" {
		t.Errorf("x listed as %+v, want pinned at 1", in)
	}

	q := d.holds(t, "test::A", "test::A::positive")
	q.Holds.Inputs = []string{"x"}
	released := answer(t, e, d, q, analysis.Budget{Depth: 3})
	expect(t, released, analysis.ClaimViolated, analysis.Witnessed)
	if x := witnessInput(t, released, "x"); !strings.HasPrefix(x.Written, "-") {
		t.Errorf("witness x = %q, want a value at or below -5", x.Written)
	}
	if in := resultInput(t, released, "x"); !in.Free {
		t.Errorf("x listed as %+v, want free", in)
	}
	if in := resultInput(t, released, "n"); in.Free || in.Value != "5" {
		t.Errorf("n listed as %+v, want pinned at 5", in)
	}
}

// TestEngineRefusesInputsItCannotFree: an unbound input of a type the encoding
// cannot narrow, a release naming no feature, and one naming an output are each
// not covered naming the feature, before any query is asked.
func TestEngineRefusesInputsItCannotFree(t *testing.T) {
	// A solver that cannot run: any query asked would be the run's error.
	e := New(func() (*solve.Solver, error) { return &solve.Solver{Name: "none", Path: "/nonexistent"}, nil })
	d := indexed(t, "string.sysml", inputsSrc)
	result := answer(t, e, d, d.holds(t, "test::A", "test::A::positive"), analysis.Budget{Depth: 3})
	expect(t, result, analysis.ClaimNone, analysis.NotCovered)
	if !strings.Contains(result.Reason, "s") || !strings.Contains(result.Reason, "String") {
		t.Errorf("reason %q does not name s : String", result.Reason)
	}

	d = indexed(t, "names.sysml", strings.Replace(freeSrc, "attribute u : Natural;", "out y : Integer;", 1))
	for _, name := range []string{"nothing", "y"} {
		q := d.holds(t, "test::A", "test::A::positive")
		q.Holds.Inputs = []string{name}
		result := answer(t, e, d, q, analysis.Budget{Depth: 3})
		expect(t, result, analysis.ClaimNone, analysis.NotCovered)
		if !strings.Contains(result.Reason, name+" free") {
			t.Errorf("releasing %s: reason %q does not name it as an input refused", name, result.Reason)
		}
	}
}

// TestEngineRefusesAnAssumptionItCannotTranslate: an assumption over a construct
// outside the translatable subset is refused naming it, before any query.
func TestEngineRefusesAnAssumptionItCannotTranslate(t *testing.T) {
	e := New(func() (*solve.Solver, error) { return &solve.Solver{Name: "none", Path: "/nonexistent"}, nil })
	src := strings.Replace(freeSrc, "constraint neg { n < 0 }", "constraint neg { (n, x) == (1, 2) }", 1)
	d := indexed(t, "untranslatable.sysml", src)
	q := d.holds(t, "test::A", "test::A::positive")
	q.Holds.Assume = []*symbols.Symbol{lookup(t, d.idx, "test::A::neg")}
	result := answer(t, e, d, q, analysis.Budget{Depth: 3})
	expect(t, result, analysis.ClaimNone, analysis.NotCovered)
	if !strings.Contains(result.Reason, "neg") {
		t.Errorf("reason %q does not name the assumption", result.Reason)
	}
}

// TestEngineWitnessWithInputsReplaysThroughTheStart: the witness of a violation
// over free inputs, written and read back as a witness file, drives the
// interpreter to the violation it claims; one naming a feature the model lacks
// is refused naming it.
func TestEngineWitnessWithInputsReplaysThroughTheStart(t *testing.T) {
	e := engine(t)
	d := indexed(t, "replay.sysml", freeSrc)
	result := answer(t, e, d, d.holds(t, "test::A", "test::A::positive"), analysis.Budget{Depth: 3})
	expect(t, result, analysis.ClaimViolated, analysis.Witnessed)
	text := runtime.Witness{Inputs: result.Witness.Inputs, Choices: result.Witness.Choices}.String()
	if !strings.HasPrefix(text, "input n = ") {
		t.Fatalf("witness text does not open with the input line:\n%s", text)
	}
	read, err := runtime.ParseWitness(text)
	if err != nil {
		t.Fatalf("read the witness back: %v", err)
	}
	m, err := d.model.Semantics()
	if err != nil {
		t.Fatal(err)
	}
	replay := func(w runtime.Witness) error {
		ctx := runtime.NewContext(m, 10000)
		if err := ctx.SetSchedule(runtime.ReplayOf(w)); err != nil {
			return err
		}
		exec, err := ctx.CreateActionExecutor(lookup(t, d.idx, "test::A"))
		if err != nil {
			return err
		}
		defer exec.Release()
		for exec.State() != runtime.StateCompleted {
			if err := exec.Step(); err != nil {
				return err
			}
			if ok, err := exec.Holds(lookup(t, d.idx, "test::A::positive"), nil); err != nil {
				return err
			} else if !ok {
				return errors.New("positive neither holds nor is violated")
			}
		}
		return nil
	}
	var violation *runtime.ViolationError
	if err := replay(read); !errors.As(err, &violation) {
		t.Fatalf("replaying the witness: %v, want the violation it claims", err)
	}

	read.Inputs = append(read.Inputs, runtime.InputTaken{Feature: "absent", Written: "1"})
	var refused *runtime.WitnessInputError
	if err := replay(read); !errors.As(err, &refused) || refused.Feature != "absent" || !errors.Is(err, runtime.ErrWitnessInput) {
		t.Fatalf("replaying a witness naming an absent feature: %v, want a WitnessInputError naming it", err)
	}
}

// TestEngineWitnessWithoutInputsReplaysAsBefore: a witness carrying only choice
// lines replays through the same policy, its inputs left to the model.
func TestEngineWitnessWithoutInputsReplaysAsBefore(t *testing.T) {
	e := engine(t)
	d := indexed(t, "choices.sysml", conditionsSrc)
	result := answer(t, e, d, d.holds(t, "test::A", "test::A::positive"), analysis.Budget{Depth: 4})
	expect(t, result, analysis.ClaimViolated, analysis.Witnessed)
	if len(result.Witness.Inputs) != 0 {
		t.Fatalf("a run with every input bound carries inputs: %v", result.Witness.Inputs)
	}
	if len(result.Inputs) != 1 || result.Inputs[0].Free || result.Inputs[0].Value != "1" {
		t.Errorf("inputs %v, want x pinned at 1", result.Inputs)
	}
	if result.Question.Free.Has(analysis.FreeInputs) {
		t.Errorf("the answered question says inputs were free: %v", result.Question.Free)
	}
	text := runtime.Witness{Choices: result.Witness.Choices}.String()
	if strings.Contains(text, "input ") {
		t.Errorf("witness text carries an input line:\n%s", text)
	}
	if _, err := runtime.ParseWitness(text); err != nil {
		t.Errorf("read the witness back: %v", err)
	}
}

// TestEngineWrittenWitnessCarriesItsTrace: the witness file the engine writes
// records the trace its replay left, though the context it replayed in was not
// tracing, so the file replays as a check's witness does.
func TestEngineWrittenWitnessCarriesItsTrace(t *testing.T) {
	e := engine(t)
	d := indexed(t, "written.sysml", freeSrc)
	q := d.holds(t, "test::A", "test::A::positive")
	q.Holds.WitnessDir = t.TempDir()
	result := answer(t, e, d, q, analysis.Budget{Depth: 3})
	expect(t, result, analysis.ClaimViolated, analysis.Witnessed)
	if result.Witness.Written == "" {
		t.Fatal("no witness file was written")
	}
	content, err := os.ReadFile(result.Witness.Written)
	if err != nil {
		t.Fatal(err)
	}
	w, err := runtime.ParseWitness(string(content))
	if err != nil {
		t.Fatalf("read the witness back: %v", err)
	}
	if w.Trace == "" {
		t.Fatalf("the written witness carries no trace:\n%s", content)
	}
	if w.Property == "" {
		t.Fatalf("the written witness names no property:\n%s", content)
	}
	m, err := d.model.Semantics()
	if err != nil {
		t.Fatal(err)
	}
	fresh := func() (*runtime.Context, error) { return runtime.NewContext(m, 10000), nil }
	positive := lookup(t, d.idx, "test::A::positive")
	props := []runtime.CheckProperty{{Name: w.Property, Holds: func(_ *runtime.Context, inv *runtime.Invocation) (bool, error) {
		ok, err := inv.Actions[0].Holds(positive, nil)
		var violation *runtime.ViolationError
		if errors.As(err, &violation) {
			return false, nil
		}
		return ok, err
	}}}
	start := func(ctx *runtime.Context) (*runtime.Invocation, error) {
		exec, err := q.Holds.Start(ctx)
		if err != nil {
			return nil, err
		}
		return &runtime.Invocation{Actions: []*runtime.ActionExecutor{exec}}, nil
	}
	if _, err := runtime.Replay(context.Background(), fresh, start, w, props); err != nil {
		t.Fatalf("replaying the written witness: %v", err)
	}
}

// optionalSrc: x may hold no value, inherited from Base with its multiplicity;
// z is the action's own optional attribute. `either` holds for every Integer x
// but not where x is absent; `unread` never reads x or z; `body` reads x in the
// body, so an absent x fails the run.
const optionalSrc = `package test {
	private import ScalarValues::*;
	action def Base {
		in x : Integer[0..1];
	}
	action def A :> Base {
		attribute z : Natural[0..1];
		attribute y : Integer = 1;
		requirement either { require x >= 0 or x < 0; }
		requirement unread { require y > 0; }
		requirement body { require y > 0 or y <= 0; }
		first start;
		action set { assign y := y + 1; }
		done;
		succession first start then set;
		succession first set then done;
	}
	action def B {
		in x : Integer[0..1];
		attribute y : Integer = 1;
		requirement body { require y > 0 or y <= 0; }
		first start;
		action set { assign y := x + 1; }
		done;
		succession first start then set;
		succession first set then done;
	}
}`

// TestEngineRangesOverAnOptionalInputsAbsence: a free input whose multiplicity
// admits no value ranges over its absence too: a requirement true of every
// value yet undefined without one is violated, the witness fixing the input at
// null and replaying to the failure it claims; one that never reads it is
// proved, the report saying the input may be absent; a body reading an absent
// input fails as the interpreter's does.
func TestEngineRangesOverAnOptionalInputsAbsence(t *testing.T) {
	e := engine(t)
	d := indexed(t, "optional.sysml", optionalSrc)

	either := answer(t, e, d, d.holds(t, "test::A", "test::A::either"), analysis.Budget{Depth: 3})
	expect(t, either, analysis.ClaimViolated, analysis.Witnessed)
	if x := witnessInput(t, either, "x"); x.Written != "null" {
		t.Errorf("witness x = %q, want null: the requirement is undefined for an absent x alone", x.Written)
	}
	if in := resultInput(t, either, "x"); !in.Free || !in.Optional || in.Value != "null" || in.String() != "x = null" {
		t.Errorf("x listed as %+v (%s), want free, optional, chosen absent", in, in)
	}
	if !strings.Contains(either.Reason, "null") {
		t.Errorf("the violation %q does not say the interpreter read null", either.Reason)
	}

	unread := answer(t, e, d, d.holds(t, "test::A", "test::A::unread"), analysis.Budget{Depth: 3})
	expect(t, unread, analysis.ClaimHolds, analysis.Proved)
	if in := resultInput(t, unread, "x"); !in.Free || !in.Optional || in.String() != "x : Integer free or absent" {
		t.Errorf("x listed as %+v (%s), want `x : Integer free or absent`", in, in)
	}
	if in := resultInput(t, unread, "z"); !in.Free || !in.Optional || in.String() != "z : Natural free in >= 0 or absent" {
		t.Errorf("z listed as %+v (%s), want `z : Natural free in >= 0 or absent`", in, in)
	}
	if in := resultInput(t, unread, "y"); in.Free || in.Optional || in.Value != "1" {
		t.Errorf("y listed as %+v, want pinned at 1", in)
	}

	body := answer(t, e, d, d.holds(t, "test::B", "test::B::body"), analysis.Budget{Depth: 3})
	expect(t, body, analysis.ClaimViolated, analysis.Witnessed)
	if x := witnessInput(t, body, "x"); x.Written != "null" {
		t.Errorf("witness x = %q, want null: the body fails for an absent x alone", x.Written)
	}
	if !strings.HasPrefix(body.Reason, "at step 2:") {
		t.Errorf("the failure is reported as %q, want the body's step 2", body.Reason)
	}
}

// TestEngineWitnessWithAnAbsentInputReplays: a witness fixing an optional input
// at null, written and read back, starts the interpreter with the feature held
// empty, as a run given no value for it is; the same line on a feature that
// must hold a value is refused at the start.
func TestEngineWitnessWithAnAbsentInputReplays(t *testing.T) {
	e := engine(t)
	d := indexed(t, "absent.sysml", optionalSrc)
	q := d.holds(t, "test::B", "test::B::body")
	q.Holds.WitnessDir = t.TempDir()
	result := answer(t, e, d, q, analysis.Budget{Depth: 3})
	expect(t, result, analysis.ClaimViolated, analysis.Witnessed)
	content, err := os.ReadFile(result.Witness.Written)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(content), "input x = null\n") {
		t.Fatalf("the written witness does not open with `input x = null`:\n%s", content)
	}
	w, err := runtime.ParseWitness(string(content))
	if err != nil {
		t.Fatalf("read the witness back: %v", err)
	}
	m, err := d.model.Semantics()
	if err != nil {
		t.Fatal(err)
	}
	start := func(w runtime.Witness) (*runtime.ActionExecutor, error) {
		ctx := runtime.NewContext(m, 10000)
		if err := ctx.SetSchedule(runtime.ReplayOf(w)); err != nil {
			return nil, err
		}
		return ctx.CreateActionExecutor(lookup(t, d.idx, "test::B"))
	}
	exec, err := start(w)
	if err != nil {
		t.Fatalf("start under the witness: %v", err)
	}
	defer exec.Release()
	if x, ok := exec.Data()["x"]; !ok || x.Kind != runtime.ValNull {
		t.Errorf("x held as %v, want null", x)
	}
	if err := exec.RunToCompletion(); err == nil || !strings.Contains(err.Error(), "null") {
		t.Errorf("the run under an absent x: %v, want the body failing to add to null", err)
	}

	w.Inputs = []runtime.InputTaken{{Feature: "y", Written: "null"}}
	if _, err := start(w); err == nil || !strings.Contains(err.Error(), "multiplicity") {
		t.Errorf("fixing y, which must hold a value, at null: %v, want a multiplicity refusal", err)
	}
}
