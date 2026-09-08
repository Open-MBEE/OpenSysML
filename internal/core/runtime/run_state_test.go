package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// runOutcome is what one run of choiceModel left on its context: the budget it
// spent and what it noted, as strings a run driven alone must match.
type runOutcome struct {
	steps, elements int64
	notes           string
}

func notesOf(notes []RunNote) string {
	var lines []string
	for _, n := range notes {
		lines = append(lines, n.String())
	}
	return strings.Join(lines, "\n")
}

// choiceExecutor builds an executor of choiceModel's action in a fresh context.
func choiceExecutor(t *testing.T) (*Context, *ActionExecutor) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, choiceModel))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "route", ast.DefAction)
	if sym == nil {
		t.Fatal("action not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	return ctx, exec
}

// stepPastChoice steps exec until its run has noted a choice point.
func stepPastChoice(t *testing.T, exec *ActionExecutor) {
	t.Helper()
	for i := 0; len(exec.Notes()) == 0; i++ {
		if i > 10 || exec.State() == StateCompleted {
			t.Fatal("the run noted no choice point")
		}
		if err := exec.Step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
}

// finish steps exec to completion.
func finish(t *testing.T, exec *ActionExecutor) {
	t.Helper()
	for i := 0; exec.State() != StateCompleted; i++ {
		if i > 50 {
			t.Fatal("the run did not complete in fifty steps")
		}
		if err := exec.Step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
}

// driveChoiceModel drives choiceModel to completion, calling between once the run
// has noted a choice point, and returns what the context reports of the run after.
func driveChoiceModel(t *testing.T, between func(ctx *Context, exec *ActionExecutor)) runOutcome {
	t.Helper()
	ctx, exec := choiceExecutor(t)
	stepPastChoice(t, exec)
	between(ctx, exec)
	finish(t, exec)
	return runOutcome{steps: ctx.run.steps, elements: ctx.run.elements, notes: notesOf(ctx.Notes())}
}

// A run driven call by call resumes with its own budget and notes after another
// run completes on the same context in between, and that run's are not mixed in.
func TestDrivenRunResumesItsOwnStateAcrossAWholeRun(t *testing.T) {
	alone := driveChoiceModel(t, func(*Context, *ActionExecutor) {})
	if alone.steps == 0 || !strings.Contains(alone.notes, "choice") {
		t.Fatalf("the run alone spent %d steps and noted %q; want a run that spends and chooses", alone.steps, alone.notes)
	}
	interrupted := driveChoiceModel(t, func(ctx *Context, exec *ActionExecutor) {
		if _, err := ctx.ExecuteAction(exec.action); err != nil {
			t.Fatalf("run in between: %v", err)
		}
	})
	if interrupted != alone {
		t.Errorf("driven across a whole run: %+v\nalone: %+v", interrupted, alone)
	}
}

// An expression evaluated between two calls into a driven run neither resets
// its budget nor spends it: the run alone and the run interrupted spend the same.
func TestDrivenRunResumesItsOwnStateAcrossAnEvaluation(t *testing.T) {
	alone := driveChoiceModel(t, func(*Context, *ActionExecutor) {})
	interrupted := driveChoiceModel(t, func(ctx *Context, _ *ActionExecutor) {
		if _, err := ctx.Eval(parseExpr(t, "1 + 2 + 3")); err != nil {
			t.Fatalf("evaluation in between: %v", err)
		}
	})
	if interrupted != alone {
		t.Errorf("driven across an evaluation: %+v\nalone: %+v", interrupted, alone)
	}
}

// Two runs driven turn and turn about each keep their own budget and notes.
func TestInterleavedDrivenRunsKeepTheirOwnState(t *testing.T) {
	alone := driveChoiceModel(t, func(*Context, *ActionExecutor) {})
	ctx, first := choiceExecutor(t)
	second, err := ctx.CreateActionExecutor(first.action)
	if err != nil {
		t.Fatalf("create second executor: %v", err)
	}
	runs := []struct {
		name string
		exec *ActionExecutor
	}{{"first", first}, {"second", second}}
	for i := 0; first.State() != StateCompleted || second.State() != StateCompleted; i++ {
		if i > 50 {
			t.Fatal("the runs did not complete in fifty rounds")
		}
		for _, run := range runs {
			name, exec := run.name, run.exec
			if exec.State() == StateCompleted {
				continue
			}
			if err := exec.Step(); err != nil {
				t.Fatalf("%s: step %d: %v", name, i, err)
			}
			if ctx.run.steps != exec.driven.state.steps || notesOf(ctx.Notes()) != notesOf(exec.Notes()) {
				t.Fatalf("%s: after its step the context reports %d steps and %q, the run has %d and %q",
					name, ctx.run.steps, notesOf(ctx.Notes()), exec.driven.state.steps, notesOf(exec.Notes()))
			}
		}
	}
	for _, run := range runs {
		name, exec := run.name, run.exec
		got := runOutcome{steps: exec.driven.state.steps, elements: exec.driven.state.elements, notes: notesOf(exec.Notes())}
		if got != alone {
			t.Errorf("%s interleaved: %+v\nalone: %+v", name, got, alone)
		}
	}
}

// The step budget bounds each driven run on its own: a run that spent nearly
// all of it fails, not one resumed after it.
func TestStepBudgetIsTheDrivenRunsOwn(t *testing.T) {
	alone := driveChoiceModel(t, func(*Context, *ActionExecutor) {})

	// The whole budget spent by another run leaves the paused run its own.
	ctx, exec := choiceExecutor(t)
	ctx.maxSteps = alone.steps
	stepPastChoice(t, exec)
	spender, err := ctx.CreateActionExecutor(exec.action)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	if err := spender.RunToCompletion(); err != nil {
		t.Fatalf("run in between: %v", err)
	}
	finish(t, exec)
	if ctx.run.steps != alone.steps {
		t.Errorf("the resumed run spent %d steps, want %d", ctx.run.steps, alone.steps)
	}

	// A run that exhausted its budget fails even when a run in between spent nothing.
	ctx, exec = choiceExecutor(t)
	ctx.maxSteps = alone.steps - 1
	stepPastChoice(t, exec)
	if _, err := ctx.Eval(parseExpr(t, "1")); err != nil {
		t.Fatalf("evaluation in between: %v", err)
	}
	err = nil
	for i := 0; err == nil && exec.State() != StateCompleted; i++ {
		if i > 50 {
			t.Fatal("the run neither completed nor failed in fifty steps")
		}
		err = exec.Step()
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("Step() = %v, want %v for the run that spent the budget", err, ErrStepLimitExceeded)
	}
	// The failed run's budget is its own: an evaluation after it has a fresh one.
	if _, err := ctx.Eval(parseExpr(t, "1")); err != nil {
		t.Errorf("evaluation after the failed run: %v", err)
	}
}

// A run begun inside another shares the enclosing run's state, as does one begun
// inside a probe, whose spending the probe undoes.
func TestNestedRunSharesTheEnclosingState(t *testing.T) {
	ctx, outer := choiceExecutor(t)
	stepPastChoice(t, outer)
	enclosing := ctx.run
	end := outer.ctx.beginExecutorRun(&outer.driven)
	nested, err := ctx.CreateActionExecutor(outer.action)
	if err != nil {
		t.Fatalf("create nested executor: %v", err)
	}
	if err := nested.RunToCompletion(); err != nil {
		t.Fatalf("nested run: %v", err)
	}
	end()
	if nested.driven.state != enclosing || ctx.run != enclosing {
		t.Fatal("a run begun inside another did not share its state")
	}
	if len(enclosing.notes) < 3 {
		t.Errorf("the enclosing run holds %d notes, want its own and the nested run's", len(enclosing.notes))
	}

	ctx, outer = choiceExecutor(t)
	stepPastChoice(t, outer)
	enclosing = ctx.run
	steps, notes := enclosing.steps, len(enclosing.notes)
	endProbe := ctx.beginProbe()
	probed, err := ctx.CreateActionExecutor(outer.action)
	if err != nil {
		t.Fatalf("create probed executor: %v", err)
	}
	if err := probed.RunToCompletion(); err != nil {
		t.Fatalf("probed run: %v", err)
	}
	endProbe()
	if probed.driven.state != enclosing || ctx.run != enclosing {
		t.Fatal("a run begun inside a probe did not share the enclosing state")
	}
	if enclosing.steps != steps || len(enclosing.notes) != notes {
		t.Errorf("the probe left %d steps and %d notes, want %d and %d", enclosing.steps, len(enclosing.notes), steps, notes)
	}
}

// memoModel reads a calc usage twice in one block, either side of a node a
// breakpoint can pause at, so the second read answers from the first's evaluation.
const memoModel = `package test {
	private import ScalarValues::*;
	calc def Twice { in k : Real; out d = k * 2.0; }
	action outer {
		attribute base : Real = 3.0;
		calc t : Twice { in k = base; }
		out attribute total : Real = 0.0;
		first start;
		then action choose {
			if base > 0.0 {
				assign total := total + t.d;
				action q { assign total := total + 1.0; }
				assign total := total + t.d;
			}
		}
		then done;
	}
}`

// A calc usage read in an activation still open across a pause answers from the
// evaluation the paused run made, whatever ran on the context in between.
func TestPausedActivationKeepsItsCalcUsageEvaluations(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, memoModel))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	trace := NewTraceRecorder()
	exec.SetTrace(trace)
	exec.SetBreakpoint("q")
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to the breakpoint: %v", err)
	}
	if got := exec.PausedAt(); got != "q" {
		t.Fatalf("PausedAt() = %q, want q", got)
	}

	ctx.SetTrace(nil)
	if _, err := ctx.ExecuteAction(sym); err != nil {
		t.Fatalf("run in between: %v", err)
	}
	exec.SetTrace(trace)

	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if total := exec.Results()["total"]; total.Const.Real != 13.0 {
		t.Errorf("total = %v, want 13", total)
	}
	if got := strings.Count(trace.String(), "enter calc test::outer::t"); got != 1 {
		t.Errorf("the paused run ran the usage's body %d times, want once:\n%s", got, trace.String())
	}
	if !strings.Contains(trace.String(), "reuse calc test::outer::t") {
		t.Errorf("the read after the pause did not reuse the evaluation:\n%s", trace.String())
	}
	if len(ctx.run.calcUsageRuns) != 0 {
		t.Errorf("%d activation(s) still held after the run", len(ctx.run.calcUsageRuns))
	}
}

// pausedAtActivation drives the memo model to its breakpoint, an activation open.
func pausedAtActivation(t *testing.T) (*Context, *ActionExecutor, *symbols.Symbol) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, memoModel))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}
	exec.SetBreakpoint("q")
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to the breakpoint: %v", err)
	}
	if len(exec.driven.state.calcUsageRuns) == 0 {
		t.Fatal("the paused run holds no activation")
	}
	return ctx, exec, sym
}

// Releasing a paused run ends its open activations in its own state and leaves
// the run the context reports as it was.
func TestReleaseEndsThePausedRunsActivations(t *testing.T) {
	ctx, exec, sym := pausedAtActivation(t)
	if _, err := ctx.ExecuteAction(sym); err != nil {
		t.Fatalf("run in between: %v", err)
	}
	between, notes := ctx.run, notesOf(ctx.Notes())
	exec.Release()
	if n := len(exec.driven.state.calcUsageRuns); n != 0 {
		t.Errorf("%d activation(s) still held after the release", n)
	}
	if ctx.run != between || notesOf(ctx.Notes()) != notes {
		t.Errorf("the release changed the run the context reports: notes %q, want %q", notesOf(ctx.Notes()), notes)
	}
}

// A release inside another run still ends the paused run's own activations, not the
// enclosing run's.
func TestReleaseInsideAnotherRunEndsThePausedRunsOwn(t *testing.T) {
	ctx, exec, _ := pausedAtActivation(t)
	end := ctx.beginRun()
	outer := ctx.run
	outer.calcUsageRuns[1] = map[calcUsageKey]*calcRun{}
	exec.Release()
	if n := len(exec.driven.state.calcUsageRuns); n != 0 {
		t.Errorf("%d activation(s) still held after the release", n)
	}
	if ctx.run != outer || len(outer.calcUsageRuns) != 1 {
		t.Errorf("the release touched the enclosing run: %d activation(s) held, want 1", len(outer.calcUsageRuns))
	}
	end()
}
