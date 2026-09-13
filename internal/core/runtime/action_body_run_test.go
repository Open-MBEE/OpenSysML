package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// The steps of a run whose work never pauses leave no work behind: a long run
// keeps no paused frame per step.
func TestStepsOfARunKeepNoPausedWork(t *testing.T) {
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
	if ctx.body != nil {
		t.Error("a body is still on the stack after the run left")
	}
	for _, token := range exec.tokens {
		if token.body != nil {
			t.Errorf("token %d holds work after the run completed", token.ID)
		}
	}
}

// A step a breakpoint pauses keeps the frames it paused in for as long as it is
// paused, and the steps resuming it pause it again at the breakpoint rather than run through.
func TestAPausedStepKeepsItsFrames(t *testing.T) {
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
		if len(paused.cursor) == 0 {
			t.Errorf("pass %d: the paused work holds no frame to go on from", pass)
		}
		if paused.paused.onWait || paused.paused.breakpoint != "add" {
			t.Errorf("pass %d: paused = %+v, want at breakpoint add", pass, paused.paused)
		}
		if exec.ctx.body != nil {
			t.Errorf("pass %d: a body is on the stack while the run is paused", pass)
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
	for _, token := range exec.tokens {
		if token.body != nil {
			t.Errorf("token %d holds work after the run completed", token.ID)
		}
	}
}

// A step whose body waits on the clock pauses in its frames until the clock
// reaches the wait, and the resumed step goes on from the wait.
func TestAStepWaitingOnTheClockPausesInItsFrames(t *testing.T) {
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
	if len(waiting.cursor) == 0 {
		t.Error("the work paused on the clock holds no frame to go on from")
	}
	if w := waiting.paused.wait; !waiting.paused.onWait || w.onMessage || !w.goesOn() {
		t.Errorf("paused = %+v, want on a wait on the clock that goes on", waiting.paused)
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
	for _, token := range exec.tokens {
		if token.body != nil {
			t.Errorf("token %d holds work after the run completed", token.ID)
		}
	}
}
