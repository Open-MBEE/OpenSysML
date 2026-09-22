package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const quiescentLampSource = `
	private import ScalarValues::*;
	private import SI::*;
	attribute def go;
	state def Lamp {
		entry; then off;
		state off;
		transition first off accept go then on;
		state on;
	}
	part def Bulb { exhibit state lamp : Lamp; }
`

const quiescentTimerSource = `
	private import ScalarValues::*;
	private import SI::*;
	state def Watch {
		entry; then waiting;
		state waiting;
		transition first waiting accept after 1 [s] then done;
		state done;
	}
	part def Winder { exhibit state watch : Watch; }
`

func quiescentBulb(t *testing.T, src, doc, part string) (*Context, *Instance, *symbols.Scope) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, doc, parseAndBuild(t, src))
	root := idx.DocumentRoot(doc)
	inst, err := ctx.Instantiate(resolveSymbol(t, root, part))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	return ctx, inst, root
}

// A signal posted into a context a scan found idle is seen as pending work
// again: the next run advances the machine it wakes.
func TestQuiescenceBrokenByPostedSignal(t *testing.T) {
	ctx, bulb, root := quiescentBulb(t, quiescentLampSource, "quiescent_lamp.sysml", "Bulb")
	if ctx.quiescentAt != ctx.work {
		t.Fatalf("context not quiescent after materialization: work=%d quiescentAt=%d", ctx.work, ctx.quiescentAt)
	}
	behavior, ok := bulb.ExhibitedState()
	if !ok {
		t.Fatal("the bulb exhibits no machine")
	}
	exec := behavior.State

	msg, err := ctx.SignalMessage(resolveSymbol(t, root, "go"), nil, bulb)
	if err != nil {
		t.Fatalf("SignalMessage(go): %v", err)
	}
	ctx.PostMessage(msg)
	if ctx.quiescentAt == ctx.work {
		t.Fatal("the posted signal left the context quiescent")
	}
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent: %v", err)
	}
	if got := activeLeaf(exec); got != "on" {
		t.Fatalf("state after go = %s, want on", got)
	}
}

// A clock moved past a wait's due instant is seen as pending work again: the
// advance fires the transition the wait armed.
func TestQuiescenceBrokenByClockAdvance(t *testing.T) {
	ctx, winder, _ := quiescentBulb(t, quiescentTimerSource, "quiescent_watch.sysml", "Winder")
	if ctx.quiescentAt != ctx.work {
		t.Fatalf("context not quiescent after materialization: work=%d quiescentAt=%d", ctx.work, ctx.quiescentAt)
	}
	behavior, ok := winder.ExhibitedState()
	if !ok {
		t.Fatal("the winder exhibits no machine")
	}
	exec := behavior.State
	if _, err := ctx.Advance(2); err != nil {
		t.Fatalf("Advance(2): %v", err)
	}
	if got := activeLeaf(exec); got != "done" {
		t.Fatalf("state after advancing past the timer = %s, want done", got)
	}
}

// A run begun while the context is quiescent counts as work itself: the run
// can leave a machine's new state accepting what the bus already holds.
func TestQuiescenceBrokenByExecutorRun(t *testing.T) {
	ctx, bulb, _ := quiescentBulb(t, quiescentLampSource, "quiescent_lamp.sysml", "Bulb")
	if ctx.quiescentAt != ctx.work {
		t.Fatalf("context not quiescent after materialization: work=%d quiescentAt=%d", ctx.work, ctx.quiescentAt)
	}
	behavior, ok := bulb.ExhibitedState()
	if !ok {
		t.Fatal("the bulb exhibits no machine")
	}
	if err := behavior.State.RunToQuiescence(); err != nil {
		t.Fatalf("RunToQuiescence: %v", err)
	}
	if ctx.quiescentAt == ctx.work {
		t.Fatal("an executor run left the context quiescent")
	}
}
