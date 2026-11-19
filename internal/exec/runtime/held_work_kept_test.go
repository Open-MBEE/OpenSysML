package runtime

import (
	"reflect"
	"testing"
)

// heldAtStart is what an outermost start finds holding work, and the map it was told in.
func heldAtStart(ctx *Context) (map[*ObjectBehavior]bool, uintptr) {
	defer ctx.holdDrivenWork()()
	return ctx.heldBehaviors, reflect.ValueOf(ctx.heldBehaviors).Pointer()
}

// A scan that finds a behavior holding work is kept while nothing wakes one:
// the next start is told the same behaviors without asking each again.
func TestHeldWorkKeptUntilWorkChanges(t *testing.T) {
	ctx, bulb, root := quiescentBulb(t, quiescentLampSource, "held_lamp.sysml", "Bulb")
	behavior, ok := bulb.ExhibitedState()
	if !ok {
		t.Fatal("the bulb exhibits no machine")
	}
	if ctx.holding.holds(ctx) {
		t.Fatalf("an idle context keeps held work: %+v", ctx.holding)
	}
	msg, err := ctx.SignalMessage(resolveSymbol(t, root, "go"), nil, bulb)
	if err != nil {
		t.Fatalf("SignalMessage(go): %v", err)
	}
	ctx.PostMessage(msg)

	held, first := heldAtStart(ctx)
	if !held[behavior] || len(held) != 1 {
		t.Fatalf("a start with go posted holds %d behaviors, want the lamp alone", len(held))
	}
	if !ctx.holding.holds(ctx) || ctx.quiescent.holds(ctx) {
		t.Fatalf("the scan kept holding=%t quiescent=%t, want the held work alone", ctx.holding.holds(ctx), ctx.quiescent.holds(ctx))
	}
	if again, second := heldAtStart(ctx); second != first || !again[behavior] {
		t.Fatal("a second start with nothing changed scanned the behaviors again")
	}

	ctx.PostMessage(msg)
	if ctx.holding.holds(ctx) {
		t.Fatal("a posted signal left the held work kept")
	}
	if again, second := heldAtStart(ctx); second == first || !again[behavior] {
		t.Fatal("a start after a posted signal was told the behaviors of the scan before it")
	}
}

// Once the machine takes what it held, the kept answer no longer stands: the
// next start finds nothing held, and the lamp is where the signal put it.
func TestHeldWorkNotKeptPastTheRunThatTakesIt(t *testing.T) {
	ctx, bulb, root := quiescentBulb(t, quiescentLampSource, "held_lamp_taken.sysml", "Bulb")
	behavior, _ := bulb.ExhibitedState()
	msg, err := ctx.SignalMessage(resolveSymbol(t, root, "go"), nil, bulb)
	if err != nil {
		t.Fatalf("SignalMessage(go): %v", err)
	}
	ctx.PostMessage(msg)
	if held, _ := heldAtStart(ctx); !held[behavior] {
		t.Fatal("a start with go posted does not hold the lamp")
	}
	if err := behavior.State.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent: %v", err)
	}
	if ctx.holding.holds(ctx) {
		t.Fatal("the run that took the signal left the held work kept")
	}
	if held, _ := heldAtStart(ctx); len(held) != 0 {
		t.Fatalf("a start after the lamp took go holds %d behaviors, want none", len(held))
	}
	if got := activeLeaf(behavior.State); got != "on" {
		t.Fatalf("state after go = %s, want on", got)
	}
}

// A scan that read the objects' data is not kept past a write.
func TestHeldWorkThatReadDataNotKeptPastAWrite(t *testing.T) {
	ctx, bulb, root := quiescentBulb(t, quiescentLampSource, "held_lamp_data.sysml", "Bulb")
	msg, err := ctx.SignalMessage(resolveSymbol(t, root, "go"), nil, bulb)
	if err != nil {
		t.Fatalf("SignalMessage(go): %v", err)
	}
	ctx.PostMessage(msg)
	heldAtStart(ctx)
	ctx.holding.readsData = true
	if !ctx.holding.holds(ctx) {
		t.Fatal("the held work is not kept with nothing written")
	}
	ctx.writes++
	if ctx.holding.holds(ctx) {
		t.Fatal("held work that read data is kept past a write")
	}
}

// heldLamp is a bulb whose lamp holds a posted go, with that answer kept.
func heldLamp(t *testing.T, doc string) (*Context, *Instance, *ObjectBehavior) {
	t.Helper()
	ctx, bulb, root := quiescentBulb(t, quiescentLampSource, doc, "Bulb")
	behavior, ok := bulb.ExhibitedState()
	if !ok {
		t.Fatal("the bulb exhibits no machine")
	}
	msg, err := ctx.SignalMessage(resolveSymbol(t, root, "go"), nil, bulb)
	if err != nil {
		t.Fatalf("SignalMessage(go): %v", err)
	}
	ctx.PostMessage(msg)
	if held, _ := heldAtStart(ctx); !held[behavior] || !ctx.holding.holds(ctx) {
		t.Fatal("a start with go posted does not keep the lamp as holding work")
	}
	return ctx, bulb, behavior
}

// Each change that can leave a behavior holding work lets go of the kept answer,
// as it does of the all-idle one.
func TestHeldWorkNotKeptPastWhatCanWakeABehavior(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(t *testing.T, ctx *Context, bulb *Instance, lamp *ObjectBehavior)
	}{
		{"the clock moving", func(_ *testing.T, ctx *Context, _ *Instance, _ *ObjectBehavior) {
			ctx.setClock(ctx.clock.now + 1)
		}},
		{"an event queued", func(_ *testing.T, _ *Context, _ *Instance, lamp *ObjectBehavior) {
			lamp.State.enqueue(Event{})
		}},
		{"a call into an executor", func(_ *testing.T, ctx *Context, _ *Instance, lamp *ObjectBehavior) {
			ctx.beginExecutorRun(&lamp.State.driven)()
		}},
		{"a behavior attached", func(t *testing.T, ctx *Context, bulb *Instance, _ *ObjectBehavior) {
			if _, err := ctx.Instantiate(bulb.Type); err != nil {
				t.Fatalf("Instantiate: %v", err)
			}
		}},
		{"a behavior dropped", func(_ *testing.T, ctx *Context, _ *Instance, lamp *ObjectBehavior) {
			ctx.forgetBehaviors([]*ObjectBehavior{lamp})
		}},
		{"a journal rolled back", func(_ *testing.T, ctx *Context, _ *Instance, _ *ObjectBehavior) {
			_, rollback := ctx.beginJournal()
			rollback()
		}},
		{"a snapshot restored", func(t *testing.T, ctx *Context, _ *Instance, _ *ObjectBehavior) {
			snap, err := ctx.Snapshot()
			if err != nil {
				t.Fatalf("Snapshot: %v", err)
			}
			snap.Restore()
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, bulb, lamp := heldLamp(t, "held_lamp_woken.sysml")
			tc.change(t, ctx, bulb, lamp)
			if ctx.holding.holds(ctx) {
				t.Fatalf("%s left the held work kept", tc.name)
			}
		})
	}
}

// A behavior mid-call is asked at every start: its state moves without a mark
// moving, so one that holds work and is not among those kept forces a scan.
func TestHeldWorkAsksABehaviorMidCallAgain(t *testing.T) {
	ctx, _, lamp := heldLamp(t, "held_lamp_midcall.sysml")
	none := map[*ObjectBehavior]bool{}
	if ctx.wokeMidCall(none) {
		t.Fatal("a settled lamp is asked again")
	}
	run := &lamp.State.driven
	run.active++
	ctx.onStack = append(ctx.onStack, run)
	defer func() { run.active--; ctx.onStack = ctx.onStack[:len(ctx.onStack)-1] }()
	if !lamp.midCall() || !ctx.wokeMidCall(none) {
		t.Fatal("a lamp mid-call holding go is not found against an empty kept set")
	}
	if ctx.wokeMidCall(map[*ObjectBehavior]bool{lamp: true}) {
		t.Fatal("a lamp mid-call already among those kept forces a scan")
	}
}

const heldAlarmSource = quiescentLampSource + `
	part def Sensor {
		attribute level : Integer = 0;
		exhibit state alarm {
			entry; then watching;
			state watching;
			accept when level > 0 then tripped;
			state tripped;
		}
	}
	part bulb : Bulb;
	part sensor : Sensor;
`

// A write that raises a change condition leaves its machine holding work; the
// probe that evaluated it moved work, so the next start scans and finds it.
func TestHeldWorkNotKeptPastAWriteThatRaisesAChangeCondition(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "held_alarm.sysml", parseAndBuild(t, heldAlarmSource))
	root := idx.DocumentRoot("held_alarm.sysml")
	bulb, err := ctx.occurrenceOf(resolveSymbol(t, root, "bulb"))
	if err != nil {
		t.Fatalf("occurrenceOf(bulb): %v", err)
	}
	sensor, err := ctx.occurrenceOf(resolveSymbol(t, root, "sensor"))
	if err != nil {
		t.Fatalf("occurrenceOf(sensor): %v", err)
	}
	alarm := sensor.ExhibitedStates()[0]
	lamp := bulb.ExhibitedStates()[0]
	if err := alarm.State.RunToCompletion(); err != nil {
		t.Fatalf("initial run of the alarm: %v", err)
	}
	msg, err := ctx.SignalMessage(resolveSymbol(t, root, "go"), nil, bulb)
	if err != nil {
		t.Fatalf("SignalMessage(go): %v", err)
	}
	ctx.PostMessage(msg)
	if held, _ := heldAtStart(ctx); !held[lamp] || held[alarm] || !ctx.holding.holds(ctx) {
		t.Fatalf("a start with go posted keeps lamp %t alarm %t, want the lamp alone", held[lamp], held[alarm])
	}

	ctx.holding.readsData = false
	if err := sensor.SetFeatureValue(ctx, "level", intArgument(1)); err != nil {
		t.Fatalf("SetFeatureValue(level): %v", err)
	}
	if got := activeLeaf(alarm.State); got != "watching" || !alarm.hasPendingWork() {
		t.Fatalf("the alarm is %s, holding work %t, want watching with its condition risen", got, alarm.hasPendingWork())
	}
	if ctx.holding.holds(ctx) {
		t.Fatal("the write that raised the alarm's condition left the held work kept")
	}
	if held, _ := heldAtStart(ctx); !held[lamp] || !held[alarm] {
		t.Fatalf("the start after the write holds lamp %t alarm %t, want both", held[lamp], held[alarm])
	}
}
