package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// The steps of a run whose work never pauses all run on one body coroutine, which
// the run ends when it leaves, so a long run makes no coroutine per step.
func TestStepsOfARunShareOneBodyCoroutine(t *testing.T) {
	ctx, sym := loadAction(t, `package test {
		private import ScalarValues::*;
		action counter {
			attribute n : Integer = 0;
			first start;
			merge again;
			action bump { assign n := n + 1; }
			done;
			succession first start then again;
			succession first again then bump;
			succession first bump then check;
			decide check;
			if n < 200 then again;
			if n >= 200 then done;
		}
	}`, "counter")
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if n := exec.Results()["n"]; n.Const.Int != 200 {
		t.Fatalf("n = %v, want 200 steps of bump", n)
	}
	if made := ctx.bodyCoroutinesMade; made != 1 {
		t.Errorf("the run made %d body coroutines, want 1 shared by its steps", made)
	}
	if ctx.idleBody != nil {
		t.Error("a body coroutine is still kept after the run left")
	}
	for _, token := range exec.tokens {
		if token.body != nil {
			t.Errorf("token %d holds work after the run completed", token.ID)
		}
	}
}

// A step a breakpoint pauses keeps its coroutine for as long as it is paused, and
// the steps resuming it pause it again at the breakpoint rather than run through.
func TestAPausedStepKeepsItsBodyCoroutine(t *testing.T) {
	exec := blockDebugExecutor(t)
	exec.SetBreakpoint("add")
	for pass := int64(0); pass < 3; pass++ {
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("pass %d: RunToCompletion: %v", pass, err)
		}
		if got := exec.PausedAt(); got != "add" {
			t.Fatalf("pass %d: PausedAt() = %q, want add", pass, got)
		}
		var paused *bodyRun
		for _, token := range exec.tokens {
			if token.body != nil {
				paused = token.body
			}
		}
		if paused == nil {
			t.Fatalf("pass %d: no token holds the paused work", pass)
		}
		if paused.co == nil || paused.co.run != paused {
			t.Errorf("pass %d: the paused work does not hold its coroutine", pass)
		}
		if exec.ctx.idleBody != nil {
			t.Errorf("pass %d: a body coroutine is kept idle while the run is paused", pass)
		}
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("final run: %v", err)
	}
	if got := exec.State(); got != StateCompleted {
		t.Errorf("State() = %v, want %v", got, StateCompleted)
	}
	if n := exec.Results()["total"]; n.Const.Int != 13 {
		t.Errorf("total = %v, want 13", n)
	}
	if made := exec.ctx.bodyCoroutinesMade; made != 1 {
		t.Errorf("the run made %d body coroutines, want the one its steps share", made)
	}
}

// A step whose body waits on the clock pauses on its coroutine until the clock
// reaches the wait, and the resumed step goes on from the wait.
func TestAStepWaitingOnTheClockPausesItsBodyCoroutine(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, nestedWaitModel))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatalf("CreateActionExecutor: %v", err)
	}
	if err := exec.RunToQuiescence(); err != nil {
		t.Fatalf("RunToQuiescence: %v", err)
	}
	if exec.State() != StateWaiting {
		t.Fatalf("State() = %v, want Waiting on the nested flow's clock wait", exec.State())
	}
	var waiting *bodyRun
	for _, token := range exec.tokens {
		if token.pausedOnClock() {
			waiting = token.body
		}
	}
	if waiting == nil {
		t.Fatal("no token is paused on the clock")
	}
	if waiting.co == nil || waiting.co.run != waiting {
		t.Error("the work paused on the clock does not hold its coroutine")
	}
	if _, err := ctx.Advance(5); err != nil {
		t.Fatalf("Advance(5): %v", err)
	}
	if got := exec.Results()["n"]; got.Kind != ValConst || got.Const.Int != 1 {
		t.Errorf("n = %v once the clock reached 5; want 1", got)
	}
	if exec.State() != StateCompleted {
		t.Errorf("State() = %v, want Completed", exec.State())
	}
	if made := ctx.bodyCoroutinesMade; made != 1 {
		t.Errorf("the run made %d body coroutines, want 1", made)
	}
	if ctx.idleBody != nil {
		t.Error("a body coroutine is still kept after the run completed")
	}
}
