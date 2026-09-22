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
	part def Bulb {
	attribute level : Integer = 0;
	exhibit state lamp : Lamp;
}
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
	if !ctx.quiescent.holds(ctx) {
		t.Fatalf("context not quiescent after materialization: work=%d memo=%+v", ctx.work, ctx.quiescent)
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
	if ctx.quiescent.holds(ctx) {
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
	if !ctx.quiescent.holds(ctx) {
		t.Fatalf("context not quiescent after materialization: work=%d memo=%+v", ctx.work, ctx.quiescent)
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
	if !ctx.quiescent.holds(ctx) {
		t.Fatalf("context not quiescent after materialization: work=%d memo=%+v", ctx.work, ctx.quiescent)
	}
	behavior, ok := bulb.ExhibitedState()
	if !ok {
		t.Fatal("the bulb exhibits no machine")
	}
	if err := behavior.State.RunToQuiescence(); err != nil {
		t.Fatalf("RunToQuiescence: %v", err)
	}
	if ctx.quiescent.holds(ctx) {
		t.Fatal("an executor run left the context quiescent")
	}
}

// A restore puts back the bus, clock and behaviors as they stood: work the
// snapshot saw is work again, so a context found idle since scans once more.
func TestQuiescenceBrokenBySnapshotRestore(t *testing.T) {
	ctx, bulb, root := quiescentBulb(t, quiescentLampSource, "quiescent_lamp.sysml", "Bulb")
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
	snap, err := ctx.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent: %v", err)
	}
	if got := activeLeaf(exec); got != "on" {
		t.Fatalf("state after go = %s, want on", got)
	}
	if err := ctx.drainObjectBehaviors(); err != nil {
		t.Fatalf("drain: %v", err)
	}
	if !ctx.quiescent.holds(ctx) {
		t.Fatalf("context not quiescent after drain: work=%d memo=%+v", ctx.work, ctx.quiescent)
	}
	snap.Restore()
	if ctx.quiescent.holds(ctx) {
		t.Fatal("the restore left the context quiescent")
	}
	if err := ctx.drainObjectBehaviors(); err != nil {
		t.Fatalf("drain after restore: %v", err)
	}
	if got := activeLeaf(exec); got != "on" {
		t.Fatalf("state after restored go = %s, want on", got)
	}
}

// A scan whose poll read the objects' data holds only until the next write;
// one that read nothing holds across writes, as quiescence is meant to.
func TestQuiescenceInvalidatedByWritesOnlyWhenTheScanReadData(t *testing.T) {
	ctx, bulb, root := quiescentBulb(t, quiescentLampSource, "quiescent_lamp.sysml", "Bulb")
	behavior, ok := bulb.ExhibitedState()
	if !ok {
		t.Fatal("the bulb exhibits no machine")
	}
	exec := behavior.State
	if !ctx.quiescent.holds(ctx) {
		t.Fatalf("context not quiescent after materialization: memo=%+v", ctx.quiescent)
	}
	if err := bulb.SetFeatureValue(ctx, "level", intArgument(3)); err != nil {
		t.Fatalf("SetFeatureValue: %v", err)
	}
	if !ctx.quiescent.holds(ctx) {
		t.Fatal("a write invalidated a scan that read no data")
	}

	// A cached poll that read data reports the read to a poll enclosing it, so
	// a scan memoizing over it marks its quiescence data-dependent.
	exec.pendingSignal()
	exec.pending.readsData = true
	exec.pending.writes = ctx.writes
	probe := &pendingMemo{}
	saved := ctx.polling
	ctx.polling = probe
	exec.pendingSignal()
	ctx.polling = saved
	if !probe.readsData {
		t.Fatal("a cached data-reading poll did not report its read to the enclosing poll")
	}

	// Marked as data-reading, the memo no longer covers the write: the work the
	// write drives rescans and records a fresh quiescence instead of standing on it.
	ctx.quiescent.readsData = true
	stale := ctx.quiescent
	if err := bulb.SetFeatureValue(ctx, "level", intArgument(4)); err != nil {
		t.Fatalf("SetFeatureValue: %v", err)
	}
	if ctx.quiescent == stale {
		t.Fatal("a write left a data-reading scan's quiescence standing")
	}
	_ = root
}
