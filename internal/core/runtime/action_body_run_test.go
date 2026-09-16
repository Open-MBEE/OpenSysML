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
		if paused.paused.onWait || paused.paused.breakpoint.name != "add" {
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

// loopWaitModel waits on the clock inside a nested flow a loop's block performs
// once per element, so a pause holds the loop's iteration and its local.
const loopWaitModel = `
	package test {
		private import SI::*;
		private import ScalarValues::*;
		action outer {
			attribute total : Integer = 0;
			first start;
			then action iterate {
				for i in 1..3 {
					action tick {
						first start;
						then action nap accept after 1 [s];
						then action add assign total := total + i;
						then done;
					}
				}
			}
			then done;
		}
	}
`

// A snapshot of a body paused inside a loop restores the loop to its iteration,
// as often as asked: the run goes on from the wait to the same end each time, and
// the restored state spells the same as the one captured.
func TestASnapshotRestoresABodyPausedInsideALoop(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, loopWaitModel))
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
	if _, err := ctx.Advance(1); err != nil {
		t.Fatalf("Advance(1): %v", err)
	}
	total := func() int64 {
		got := exec.Results()["total"]
		if got.Kind != ValConst {
			t.Fatalf("total = %v, want a constant", got)
		}
		return got.Const.Int
	}
	pausedLoop := func() *loopFrame {
		for _, token := range exec.tokens {
			if token.body == nil {
				continue
			}
			for _, f := range token.body.cursor {
				if loop, isLoop := f.(*loopFrame); isLoop {
					return loop
				}
			}
		}
		return nil
	}
	if got := total(); got != 1 {
		t.Fatalf("total = %d at t=1, want the first iteration's 1", got)
	}
	atSecond := func(loop *loopFrame) bool {
		return loop != nil && loop.locals["i"].Kind == ValConst && loop.locals["i"].Const.Int == 2
	}
	if loop := pausedLoop(); !atSecond(loop) {
		t.Fatalf("paused loop = %+v, want paused in its second iteration", loop)
	}
	spelt := (&Invocation{Actions: []*ActionExecutor{exec}}).canonicalState(nil).text
	snapshot, err := exec.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	defer snapshot.Release()
	finish := func(pass string) {
		if _, err := ctx.Advance(5); err != nil {
			t.Fatalf("%s: Advance(5): %v", pass, err)
		}
		if got := total(); got != 6 {
			t.Errorf("%s: total = %d, want 6 over three iterations", pass, got)
		}
		if exec.State() != StateCompleted {
			t.Errorf("%s: State() = %v, want Completed", pass, exec.State())
		}
		if got := ctx.Clock().Now(); got != 6 {
			t.Errorf("%s: clock = %v, want 6", pass, got)
		}
	}
	finish("first run")
	for pass := 1; pass <= 2; pass++ {
		snapshot.Restore()
		if got := total(); got != 1 {
			t.Fatalf("restore %d: total = %d, want 1", pass, got)
		}
		if loop := pausedLoop(); !atSecond(loop) {
			t.Fatalf("restore %d: paused loop = %+v, want the second iteration again", pass, loop)
		}
		if got := (&Invocation{Actions: []*ActionExecutor{exec}}).canonicalState(nil).text; got != spelt {
			t.Fatalf("restore %d: canonical state\n%s\nwant the captured one\n%s", pass, got, spelt)
		}
		finish("restore " + string(rune('0'+pass)))
	}
}
