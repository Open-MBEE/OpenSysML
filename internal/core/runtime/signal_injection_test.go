package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const lampSource = `
	private import ScalarValues::*;
	private import SI::*;
	attribute def go;
	attribute def Dim { attribute level : Integer; }
	attribute def Batch { attribute levels : Integer[2..*]; }
	state def Lamp {
		attribute brightness : Integer = 0;
		entry; then off;
		state off;
		transition off_on first off accept go then on;
		state on;
		transition on_dim first on accept d : Dim if d.level > 0 do assign brightness := d.level then dimmed;
		state dimmed;
		transition dim_out first dimmed accept after 5 [s] then off;
	}
	part def Bulb { exhibit state lamp : Lamp; }
	part def Plain;
	part plain : Plain;
`

// A message built from outside the model is typed by its definition, addressed
// to its object, and delivered by the same step a model's send is.
func TestSignalMessageDrivesTheExhibitedMachine(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "lamp.sysml", parseAndBuild(t, lampSource))
	root := idx.DocumentRoot("lamp.sysml")
	bulb, err := ctx.Instantiate(resolveSymbol(t, root, "Bulb"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	behavior, ok := bulb.ExhibitedState()
	if !ok {
		t.Fatal("the bulb exhibits no machine")
	}
	exec := behavior.State
	if exec.Performer() != bulb {
		t.Errorf("Performer() = %v, want the bulb", exec.Performer())
	}

	goSym := resolveSymbol(t, root, "go")
	msg, err := ctx.SignalMessage(goSym, nil, bulb)
	if err != nil {
		t.Fatalf("SignalMessage(go): %v", err)
	}
	if msg.Signal != goSym || msg.SignalType != "go" || msg.Object != bulb.ID || msg.Delivery != DeliverObject {
		t.Errorf("message = %+v, want go typed by its definition and addressed to the bulb", msg)
	}
	if accepted, err := exec.AcceptsMessage(msg); err != nil || !accepted {
		t.Fatalf("AcceptsMessage(go) = %v, %v; want the lamp in off to accept it", accepted, err)
	}
	if d, err := exec.Decide(msg); err != nil || len(d.Fires) != 1 || d.Fires[0] != "transition off_on" || d.Deferred {
		t.Errorf("Decide(go) = %+v, %v; want off_on firing", d, err)
	}
	ctx.PostMessage(msg)
	if !exec.HasPendingSignal() {
		t.Fatal("the posted message is not pending for the lamp")
	}
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent: %v", err)
	}
	if got := activeLeaf(exec); got != "on" {
		t.Fatalf("state after go = %s, want on", got)
	}
	if d, ok := exec.LastDispatch(); !ok || !d.Fired || d.Deferred {
		t.Errorf("LastDispatch after go = %+v, %v; want the signal fired on", d, ok)
	}

	dimSym := resolveSymbol(t, root, "Dim")
	if _, err := ctx.SignalMessage(dimSym, map[string]Value{"brightness": integerValue(1)}, bulb); !errors.Is(err, ErrSignalArgument) {
		t.Errorf("an argument Dim carries no feature for gave %v, want ErrSignalArgument", err)
	} else if !strings.Contains(err.Error(), "(it carries level)") {
		t.Errorf("the argument error does not name what Dim carries: %v", err)
	}
	if _, err := ctx.SignalMessage(dimSym, map[string]Value{"level": NewStringValue("x")}, bulb); !errors.Is(err, ErrSignalArgument) || !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("a string for Dim.level gave %v, want ErrSignalArgument wrapping ErrTypeMismatch", err)
	}
	batchSym := resolveSymbol(t, root, "Batch")
	if _, err := ctx.SignalMessage(batchSym, map[string]Value{"levels": integerValue(1)}, bulb); !errors.Is(err, ErrSignalArgument) || !errors.Is(err, ErrMultiplicityViolation) {
		t.Errorf("one value for Batch.levels[2..*] gave %v, want ErrSignalArgument wrapping ErrMultiplicityViolation", err)
	}
	if len(ctx.PendingMessages()) != 0 || activeLeaf(exec) != "on" {
		t.Errorf("a refused message left the bus or the machine changed: %d pending, state %s", len(ctx.PendingMessages()), activeLeaf(exec))
	}

	// The guard of on_dim reads the payload: deciding a level it rejects binds
	// the payload as dispatch would, finds nothing enabled, and leaves no trace —
	// no occurrence, no bound name, no cached value on the message. Dispatching
	// it anyway consumes it without a transition.
	dark, err := ctx.SignalMessage(dimSym, map[string]Value{"level": integerValue(0)}, bulb)
	if err != nil {
		t.Fatalf("SignalMessage(Dim 0): %v", err)
	}
	objects := len(ctx.instances)
	if d, err := exec.Decide(dark); err != nil || d.Enabled() {
		t.Errorf("Decide(Dim 0) = %+v, %v; want nothing enabled", d, err)
	}
	if _, bound := exec.StateData()["d"]; bound || len(ctx.instances) != objects || len(dark.Payload) != 1 {
		t.Errorf("deciding left a trace: d bound %v, %d objects (was %d), payload %v", bound, len(ctx.instances), objects, dark.Payload)
	}
	if accepted, err := exec.AcceptsMessage(dark); err != nil || !accepted {
		t.Errorf("AcceptsMessage(Dim) = %v, %v; want a guarded-out Dim taken off the bus as dispatch would", accepted, err)
	}
	ctx.PostMessage(dark)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Dim 0): %v", err)
	}
	if d, ok := exec.LastDispatch(); !ok || d.Fired || d.Deferred {
		t.Errorf("LastDispatch after a guarded-out Dim = %+v, %v; want dispatched, not fired, not deferred", d, ok)
	} else if p, isMsg := d.Event.Payload.(Message); !isMsg || p.SignalType != "Dim" {
		t.Errorf("LastDispatch carries %T %+v, want the Dim message", d.Event.Payload, d.Event.Payload)
	}
	if len(ctx.PendingMessages()) != 0 || exec.EventQueue().Len() != 0 || activeLeaf(exec) != "on" {
		t.Errorf("a guarded-out signal was not consumed: %d pending, %d queued, state %s", len(ctx.PendingMessages()), exec.EventQueue().Len(), activeLeaf(exec))
	}

	dim, err := ctx.SignalMessage(dimSym, map[string]Value{"level": integerValue(7)}, bulb)
	if err != nil {
		t.Fatalf("SignalMessage(Dim): %v", err)
	}
	if d, err := exec.Decide(dim); err != nil || len(d.Fires) != 1 || d.Fires[0] != "transition on_dim" {
		t.Errorf("Decide(Dim 7) = %+v, %v; want on_dim firing, its guard reading the payload", d, err)
	}
	ctx.PostMessage(dim)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Dim): %v", err)
	}
	if got := activeLeaf(exec); got != "dimmed" {
		t.Fatalf("state after Dim = %s, want dimmed", got)
	}
	if got := FormatValue(exec.StateData()["brightness"]); got != "7" {
		t.Errorf("brightness = %s, want 7 written by the accept's effect", got)
	}
	if d, ok := exec.LastDispatch(); !ok || !d.Fired {
		t.Errorf("LastDispatch after Dim 7 = %+v, %v; want fired", d, ok)
	}

	// A timer is now set for later; a signal in flight is still dispatched
	// first, since it is due now.
	if exec.EventQueue().Len() != 1 {
		t.Fatalf("queue holds %d events, want the dim_out timer", exec.EventQueue().Len())
	}
	if _, err := ctx.SignalMessage(resolveSymbol(t, root, "plain"), nil, bulb); !errors.Is(err, ErrNotASignal) {
		t.Errorf("a usage as a signal gave %v, want ErrNotASignal", err)
	}
}

// A behavior definition types no signal: no accept is typed by a state, action
// or calc definition, so a message named by one is refused as a usage is.
func TestSignalMessageRefusesBehaviorDefinitions(t *testing.T) {
	src := `
		private import ScalarValues::*;
		attribute def Go;
		state def Life { entry; then idle; state idle; }
		action def Kick;
		calc def Twice { in x : Integer; return : Integer = x * 2; }
		part def Lamp {
			exhibit state life : Life;
		}
		part lamp : Lamp;
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "kinds.sysml", parseAndBuild(t, src))
	root := idx.DocumentRoot("kinds.sysml")
	lamp, err := ctx.occurrenceOf(resolveSymbol(t, root, "lamp"))
	if err != nil {
		t.Fatalf("occurrenceOf(lamp): %v", err)
	}
	if _, err := ctx.SignalMessage(resolveSymbol(t, root, "Go"), nil, lamp); err != nil {
		t.Fatalf("SignalMessage(Go): %v", err)
	}
	for _, name := range []string{"Life", "Kick", "Twice"} {
		sym := resolveSymbol(t, root, name)
		if IsSignalDefinition(sym) {
			t.Errorf("IsSignalDefinition(%s %s) = true, want false", sym.Notation(), name)
		}
		if _, err := ctx.SignalMessage(sym, nil, lamp); !errors.Is(err, ErrNotASignal) {
			t.Errorf("SignalMessage(%s %s) = %v, want ErrNotASignal", sym.Notation(), name, err)
		}
	}
}

// A signal in flight is due at the current time, so stepping dispatches it
// before a timer the queue holds for later.
func TestProcessNextEventTakesAPendingSignalBeforeALaterTimer(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		state def Waiter {
			entry; then idle;
			state idle;
			transition first idle accept after 10 then late;
			transition first idle accept Kick then kicked;
			state late;
			state kicked;
		}
		attribute def Kick;
	`)
	ctx := NewContext(model, resolver, 10000)
	exec, err := ctx.CreateStateExecutorFor(resolveSymbol(t, root, "Waiter"), nil)
	if err != nil {
		t.Fatalf("CreateStateExecutorFor: %v", err)
	}
	if exec.EventQueue().Len() != 1 {
		t.Fatalf("queue holds %d events, want the timer", exec.EventQueue().Len())
	}
	msg, err := ctx.SignalMessage(resolveSymbol(t, root, "Kick"), nil, nil)
	if err != nil {
		t.Fatalf("SignalMessage: %v", err)
	}
	ctx.PostMessage(msg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent: %v", err)
	}
	if got := activeLeaf(exec); got != "kicked" {
		t.Errorf("state = %s, want kicked: the signal is due before the timer", got)
	}
	if exec.CurrentTime() != 0 {
		t.Errorf("time = %v, want 0: dispatching the signal advances no clock", exec.CurrentTime())
	}
}

// A run to completion advances time to a queued timer only once nothing due now
// is left, and a signal in flight is due now: it is dispatched before the timer,
// not once the run has advanced to it.
func TestRunToCompletionTakesAPendingSignalBeforeALaterTimer(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		state def Waiter {
			entry; then idle;
			state idle;
			transition first idle accept after 10 then late;
			transition first idle accept Kick then kicked;
			state late;
			state kicked;
			transition first kicked accept after 5 then done;
			state done;
		}
		attribute def Kick;
	`)
	ctx := NewContext(model, resolver, 10000)
	exec, err := ctx.CreateStateExecutorFor(resolveSymbol(t, root, "Waiter"), nil)
	if err != nil {
		t.Fatalf("CreateStateExecutorFor: %v", err)
	}
	if exec.EventQueue().Len() != 1 {
		t.Fatalf("queue holds %d events, want the timer", exec.EventQueue().Len())
	}
	msg, err := ctx.SignalMessage(resolveSymbol(t, root, "Kick"), nil, nil)
	if err != nil {
		t.Fatalf("SignalMessage: %v", err)
	}
	ctx.PostMessage(msg)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := activeLeaf(exec); got != "done" {
		t.Errorf("state = %s, want done: the signal is due before the timer, then kicked's own timer runs", got)
	}
	if exec.CurrentTime() != 5 {
		t.Errorf("time = %v, want 5: idle's timer was abandoned with idle, kicked's was run", exec.CurrentTime())
	}
	if n := len(ctx.PendingMessages()); n != 0 {
		t.Errorf("%d messages in flight after the run, want the signal taken", n)
	}
}

// Deciding spends the run's budget only while it lasts: the guard it evaluates
// costs the run nothing afterwards, so a dispatch the budget allows on its own
// is not failed by the preflight before it — while a guard the budget cannot
// cover still fails the preflight rather than running on.
func TestDecideRefundsTheBudgetItSpends(t *testing.T) {
	lampOn := func(t *testing.T) (*Context, *StateExecutor, Message) {
		t.Helper()
		idx, _, ctx := buildRuntimeWithLibraries(t, "lamp.sysml", parseAndBuild(t, lampSource))
		root := idx.DocumentRoot("lamp.sysml")
		bulb, err := ctx.Instantiate(resolveSymbol(t, root, "Bulb"))
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		behavior, _ := bulb.ExhibitedState()
		on, err := ctx.SignalMessage(resolveSymbol(t, root, "go"), nil, bulb)
		if err != nil {
			t.Fatalf("SignalMessage(go): %v", err)
		}
		ctx.PostMessage(on)
		if err := behavior.State.ProcessNextEvent(); err != nil {
			t.Fatalf("ProcessNextEvent(go): %v", err)
		}
		dim, err := ctx.SignalMessage(resolveSymbol(t, root, "Dim"), map[string]Value{"level": integerValue(7)}, bulb)
		if err != nil {
			t.Fatalf("SignalMessage(Dim): %v", err)
		}
		return ctx, behavior.State, dim
	}

	// What dispatching Dim costs on its own, guard and effect included.
	ctx, exec, dim := lampOn(t)
	spent := ctx.run.steps
	ctx.PostMessage(dim)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Dim): %v", err)
	}
	cost := ctx.run.steps - spent
	if cost <= 0 || activeLeaf(exec) != "dimmed" {
		t.Fatalf("dispatching Dim cost %d steps and reached %s; want a cost and dimmed", cost, activeLeaf(exec))
	}

	// A budget with exactly that left: deciding first leaves it whole, so the
	// dispatch still fits.
	ctx, exec, dim = lampOn(t)
	ctx.maxSteps = ctx.run.steps + cost
	steps, elements := ctx.run.steps, ctx.run.elements
	if d, err := exec.Decide(dim); err != nil || len(d.Fires) != 1 {
		t.Fatalf("Decide(Dim) = %+v, %v; want on_dim firing", d, err)
	}
	if ctx.run.steps != steps || ctx.run.elements != elements {
		t.Errorf("deciding charged the run %d steps and %d elements", ctx.run.steps-steps, ctx.run.elements-elements)
	}
	ctx.PostMessage(dim)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Dim) after deciding: %v", err)
	}
	if got := activeLeaf(exec); got != "dimmed" {
		t.Errorf("state after Dim = %s, want dimmed", got)
	}

	// No budget left: the guard is still bounded, and the refund still made.
	ctx, exec, dim = lampOn(t)
	ctx.maxSteps = ctx.run.steps
	steps = ctx.run.steps
	if _, err := exec.Decide(dim); !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("Decide(Dim) with no budget gave %v, want ErrStepLimitExceeded", err)
	}
	if ctx.run.steps != steps || ctx.runDepth != 0 {
		t.Errorf("a failed preflight left %d steps charged and %d runs open", ctx.run.steps-steps, ctx.runDepth)
	}
}

// A signal typed by a symbol from another scope tree over the same document —
// as a REPL command names it, while a carried-over machine resolves in the
// index's tree — is the same declaration: the machine accepts it, and the
// definition finds the machine.
func TestSignalIdentityCrossesScopeTrees(t *testing.T) {
	file := parseAndBuild(t, lampSource)
	idx, _, ctx := buildRuntimeWithLibraries(t, "lamp.sysml", file)
	bulb, err := ctx.Instantiate(resolveSymbol(t, idx.DocumentRoot("lamp.sysml"), "Bulb"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	exec := bulb.behaviors[0].State

	other := symbols.Build(file)
	goSym := resolveSymbol(t, other, "go")
	if indexed := resolveSymbol(t, idx.DocumentRoot("lamp.sysml"), "go"); indexed == goSym || indexed.Decl != goSym.Decl {
		t.Fatalf("the trees share a symbol or differ in declaration: %p vs %p", indexed, goSym)
	}
	msg, err := ctx.SignalMessage(goSym, nil, bulb)
	if err != nil {
		t.Fatalf("SignalMessage(go): %v", err)
	}
	if accepted, err := exec.AcceptsMessage(msg); err != nil || !accepted {
		t.Fatalf("AcceptsMessage(go) = %v, %v; want go typed from another tree accepted in off", accepted, err)
	}
	if d, err := exec.Decide(msg); err != nil || len(d.Fires) != 1 {
		t.Errorf("Decide(go) = %+v, %v; want off_on firing", d, err)
	}
	batch, err := ctx.SignalMessage(resolveSymbol(t, other, "Batch"), nil, bulb)
	if err != nil {
		t.Fatalf("SignalMessage(Batch): %v", err)
	}
	if accepted, err := exec.AcceptsMessage(batch); err != nil || accepted {
		t.Errorf("AcceptsMessage(Batch) = %v, %v; want it refused in off, where no transition takes it", accepted, err)
	}
	if ms := bulb.ExhibitedStatesOf(resolveSymbol(t, other, "Lamp")); len(ms) != 1 || ms[0].State != exec {
		t.Errorf("ExhibitedStatesOf(Lamp from another tree) = %v; want the lamp", ms)
	}
	if ms := bulb.ExhibitedStatesOf(resolveSymbol(t, other, "Plain")); len(ms) != 0 {
		t.Errorf("ExhibitedStatesOf(Plain) = %v; want none", ms)
	}
}

// A message on the bus that the active configuration defers, accepting it
// nowhere, is the machine's to take: Decide reports it deferred, the step
// dispatching it holds it, and it fires once a state accepting it is reached.
// One neither accepted nor deferred is left on the bus.
func TestPostedMessageTheActiveStateDefersIsHeld(t *testing.T) {
	src := `
		attribute def Ping;
		attribute def Go;
		attribute def Noise;
		state def Worker {
			entry; then busy;
			state busy { defer Ping; }
			transition first busy accept Go then ready;
			state ready;
			transition first ready accept Ping then finished;
			state finished;
		}
		part def Server { exhibit state worker : Worker; }
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "worker.sysml", parseAndBuild(t, src))
	root := idx.DocumentRoot("worker.sysml")
	server, err := ctx.Instantiate(resolveSymbol(t, root, "Server"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	exec := server.behaviors[0].State

	noise, err := ctx.SignalMessage(resolveSymbol(t, root, "Noise"), nil, server)
	if err != nil {
		t.Fatalf("SignalMessage(Noise): %v", err)
	}
	if accepted, err := exec.AcceptsMessage(noise); err != nil || accepted {
		t.Errorf("AcceptsMessage(Noise) = %v, %v; want it refused in busy, which neither accepts nor defers it", accepted, err)
	}

	ping, err := ctx.SignalMessage(resolveSymbol(t, root, "Ping"), nil, server)
	if err != nil {
		t.Fatalf("SignalMessage(Ping): %v", err)
	}
	if accepted, err := exec.AcceptsMessage(ping); err != nil || !accepted {
		t.Fatalf("AcceptsMessage(Ping) = %v, %v; want Ping, deferred in busy, taken", accepted, err)
	}
	if d, err := exec.Decide(ping); err != nil || !d.Deferred || len(d.Fires) != 0 {
		t.Errorf("Decide(Ping) = %+v, %v; want deferred and nothing firing", d, err)
	}
	ctx.PostMessage(ping)
	if !exec.HasPendingSignal() {
		t.Fatal("the posted Ping is not pending")
	}
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Ping): %v", err)
	}
	if got := activeLeaf(exec); got != "busy" {
		t.Fatalf("state after a deferred Ping = %s, want busy", got)
	}
	if d, ok := exec.LastDispatch(); !ok || d.Fired || !d.Deferred {
		t.Errorf("LastDispatch after Ping = %+v, %v; want deferred", d, ok)
	}
	if held := exec.DeferredEvents(); len(held) != 1 || len(ctx.PendingMessages()) != 0 {
		t.Fatalf("Ping held = %d, on the bus = %d; want held once and off the bus", len(held), len(ctx.PendingMessages()))
	}

	goMsg, err := ctx.SignalMessage(resolveSymbol(t, root, "Go"), nil, server)
	if err != nil {
		t.Fatalf("SignalMessage(Go): %v", err)
	}
	ctx.PostMessage(goMsg)
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Go): %v", err)
	}
	if got := activeLeaf(exec); got != "ready" {
		t.Fatalf("state after Go = %s, want ready", got)
	}
	if held := exec.DeferredEvents(); len(held) != 0 || exec.EventQueue().Len() != 1 {
		t.Fatalf("after leaving busy: held = %d, queued = %d; want Ping recalled to the queue", len(held), exec.EventQueue().Len())
	}
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(recalled Ping): %v", err)
	}
	if got := activeLeaf(exec); got != "finished" {
		t.Fatalf("state after the recalled Ping = %s, want finished", got)
	}
}

// A timer running out under a false guard fires nothing: LastDispatch says so,
// for a transition out of the single active hierarchy and for one local to an
// orthogonal region alike.
func TestTimerUnderFalseGuardIsNotReportedFired(t *testing.T) {
	cases := map[string]string{
		"simple": `
			private import ScalarValues::*;
			private import SI::*;
			state Timed {
				attribute armed : Boolean = false;
				entry; then waiting;
				state waiting;
				transition first waiting accept after 5 [s] if armed then finished;
				state finished;
			}
		`,
		"region": `
			private import ScalarValues::*;
			private import SI::*;
			state Timed parallel {
				attribute armed : Boolean = false;
				state left {
					entry; then waiting;
					state waiting;
					transition first waiting accept after 5 [s] if armed then finished;
					state finished;
				}
				state right {
					entry; then idle;
					state idle;
				}
			}
		`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "timed.sysml", parseAndBuild(t, src))
			root := idx.DocumentRoot("timed.sysml")
			exec, err := newStateExecutor(ctx, resolveSymbol(t, root, "Timed"), nil)
			if err != nil {
				t.Fatalf("newStateExecutor: %v", err)
			}
			if err := exec.initialize(); err != nil {
				t.Fatalf("initialize: %v", err)
			}
			if exec.EventQueue().Len() != 1 {
				t.Fatalf("queued after entering waiting: %d events, want the timer", exec.EventQueue().Len())
			}
			if err := exec.ProcessNextEvent(); err != nil {
				t.Fatalf("ProcessNextEvent(timer): %v", err)
			}
			if !exec.isActive(stateNamed(t, exec, "waiting")) {
				t.Fatalf("waiting left on a timer whose guard is false; active: %v", exec.ActiveStates())
			}
			d, ok := exec.LastDispatch()
			if !ok || d.Event.Type != EventTime {
				t.Fatalf("LastDispatch = %+v, %v; want the timer dispatched", d, ok)
			}
			if d.Fired || d.Deferred {
				t.Errorf("LastDispatch = %+v; want neither fired nor deferred", d)
			}
		})
	}
}

// A guard reaching a package-level occurrence not yet built materializes it, and
// building it would start the behaviors its type exhibits — here a machine whose
// entry pokes the very door being decided. Decide leaves none of that behind: no
// object, no behavior attached or pending, no message sent, the door unmoved. The
// dispatch that follows builds the object for real and runs it.
func TestDecideLeavesNoBehaviorAGuardMaterializes(t *testing.T) {
	src := `
		private import ScalarValues::*;
		attribute def Poke;
		state def Ticker {
			entry; then idle;
			state idle { entry send Poke() to door; }
		}
		part def Sensor {
			attribute level : Integer = 3;
			exhibit state ticker : Ticker;
		}
		part sensor : Sensor;
		state def Gate {
			entry; then shut;
			state shut;
			transition first shut accept Poke if sensor.level > 0 then open;
			state open;
		}
		part def Door { exhibit state gate : Gate; }
		part door : Door;
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "gate.sysml", parseAndBuild(t, src))
	root := idx.DocumentRoot("gate.sysml")
	door, err := ctx.occurrenceOf(resolveSymbol(t, root, "door"))
	if err != nil {
		t.Fatalf("occurrenceOf(door): %v", err)
	}
	gate, _ := door.ExhibitedState()
	poke, err := ctx.SignalMessage(resolveSymbol(t, root, "Poke"), nil, door)
	if err != nil {
		t.Fatalf("SignalMessage(Poke): %v", err)
	}
	sensor := resolveSymbol(t, root, "sensor")
	if _, built := ctx.occurrences[sensor]; built {
		t.Fatal("sensor is built before anything reads it")
	}
	instances, behaviors, pending, messages := len(ctx.instances), len(ctx.objectBehaviors), len(ctx.pendingBehaviors), len(ctx.messages)

	if d, err := gate.State.Decide(poke); err != nil || len(d.Fires) != 1 {
		t.Fatalf("Decide(Poke) = %+v, %v; want the guarded transition firing", d, err)
	}
	if _, built := ctx.occurrences[sensor]; built {
		t.Error("the sensor the guard read outlives the decision")
	}
	if len(ctx.instances) != instances || len(ctx.objectBehaviors) != behaviors || len(ctx.pendingBehaviors) != pending || len(ctx.messages) != messages {
		t.Errorf("after Decide: %d objects, %d behaviors (%d pending), %d messages; want %d, %d (%d), %d as before",
			len(ctx.instances), len(ctx.objectBehaviors), len(ctx.pendingBehaviors), len(ctx.messages),
			instances, behaviors, pending, messages)
	}
	if _, ok := ctx.nextRunnableBehavior(); ok {
		t.Error("a behavior is left runnable after Decide")
	}
	if got := activeLeaf(gate.State); got != "shut" {
		t.Fatalf("state after Decide = %s, want shut: the decision moved the machine", got)
	}

	ctx.PostMessage(poke)
	if err := gate.State.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Poke): %v", err)
	}
	if got := activeLeaf(gate.State); got != "open" {
		t.Fatalf("state after Poke = %s, want open", got)
	}
	id, built := ctx.occurrences[sensor]
	if !built {
		t.Fatal("dispatching Poke did not build the sensor its guard reads")
	}
	ticker, ok := ctx.instances[id].ExhibitedState()
	if !ok || activeLeaf(ticker.State) != "idle" {
		t.Fatalf("the built sensor runs no ticker at idle: ok=%v", ok)
	}
}

// A message routed to a port of the machine is deferred as one addressed to the
// machine is: busy defers Ping, which arrives at inPort while busy is active
// and no transition accepts it there, so the machine takes and holds it; once a
// timer moves it to ready the held Ping is recalled and taken via inPort.
func TestPortRoutedMessageTheActiveStateDefersIsHeld(t *testing.T) {
	src := `
		private import SI::*;
		item def Ping;
		port def PingPort { in item ping : Ping; }
		state Radio {
			port outPort : PingPort;
			port inPort : PingPort;
			connect outPort to inPort;
			entry; then busy;
			state busy {
				defer Ping;
				entry send Ping() via outPort;
			}
			transition first busy accept after 5 [s] then ready;
			state ready;
			transition first ready accept Ping via inPort then finished;
			state finished;
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "radio.sysml", parseAndBuild(t, src))
	root := idx.DocumentRoot("radio.sysml")
	exec, err := newStateExecutor(ctx, resolveSymbol(t, root, "Radio"), nil)
	if err != nil {
		t.Fatalf("newStateExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	pending := ctx.PendingMessages()
	if len(pending) != 1 || pending[0].Port != "inPort" {
		t.Fatalf("after entering busy, on the bus: %+v; want Ping routed to inPort", pending)
	}
	if accepted, err := exec.AcceptsMessage(pending[0]); err != nil || !accepted {
		t.Fatalf("AcceptsMessage(Ping at inPort) = %v, %v; want it taken, deferred in busy", accepted, err)
	}
	if d, err := exec.Decide(pending[0]); err != nil || !d.Deferred || len(d.Fires) != 0 {
		t.Errorf("Decide(Ping via inPort) = %+v, %v; want deferred and nothing firing", d, err)
	}
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Ping): %v", err)
	}
	if got := activeLeaf(exec); got != "busy" {
		t.Fatalf("state after a deferred Ping = %s, want busy", got)
	}
	if held := exec.DeferredEvents(); len(held) != 1 || len(ctx.PendingMessages()) != 0 {
		t.Fatalf("Ping held = %d, on the bus = %d; want held once and off the bus", len(held), len(ctx.PendingMessages()))
	}
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(timer): %v", err)
	}
	if got := activeLeaf(exec); got != "ready" {
		t.Fatalf("state after the timer = %s, want ready", got)
	}
	if held := exec.DeferredEvents(); len(held) != 0 || exec.EventQueue().Len() != 1 {
		t.Fatalf("after leaving busy: held = %d, queued = %d; want Ping recalled to the queue", len(held), exec.EventQueue().Len())
	}
	if err := exec.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(recalled Ping): %v", err)
	}
	if got := activeLeaf(exec); got != "finished" {
		t.Fatalf("state after the recalled Ping = %s, want finished", got)
	}
}

// A variant that is a value stands for no object, so its selection is recorded
// alone; Decide reading it through a guard leaves that record as it found it,
// and the dispatch that follows makes the selection for real.
func TestDecideLeavesNoValueVariantSelectionAGuardMakes(t *testing.T) {
	src := `
		private import ScalarValues::*;
		attribute def Poke;
		part def Car {
			variation attribute power : Real {
				variant attribute strong = 150.0;
				variant attribute weak = 120.0;
			}
			exhibit state gate {
				entry; then shut;
				state shut;
				transition first shut accept Poke if power > 130.0 then open;
				state open;
			}
		}
		part car : Car { attribute :>> power = power::%s; }
	`
	for variant, fires := range map[string]bool{"strong": true, "weak": false} {
		t.Run(variant, func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "car.sysml", parseAndBuild(t, fmt.Sprintf(src, variant)))
			root := idx.DocumentRoot("car.sysml")
			car, err := ctx.occurrenceOf(resolveSymbol(t, root, "car"))
			if err != nil {
				t.Fatalf("occurrenceOf(car): %v", err)
			}
			gate, _ := car.ExhibitedState()
			poke, err := ctx.SignalMessage(resolveSymbol(t, root, "Poke"), nil, car)
			if err != nil {
				t.Fatalf("SignalMessage(Poke): %v", err)
			}
			selection := variantSelection{owner: car.ID, variation: "power"}
			if chosen, ok := ctx.selectedVariants[selection]; ok {
				t.Fatalf("power selected %q before anything read it", chosen)
			}
			selections := len(ctx.selectedVariants)

			d, err := gate.State.Decide(poke)
			if err != nil || (len(d.Fires) == 1) != fires {
				t.Fatalf("Decide(Poke) = %+v, %v; want firing=%v", d, err, fires)
			}
			if chosen, ok := ctx.selectedVariants[selection]; ok || len(ctx.selectedVariants) != selections {
				t.Errorf("after Decide: power selected %q (%v), %d selections; want none made, %d as before",
					chosen, ok, len(ctx.selectedVariants), selections)
			}

			ctx.PostMessage(poke)
			if err := gate.State.ProcessNextEvent(); err != nil {
				t.Fatalf("ProcessNextEvent(Poke): %v", err)
			}
			want := "shut"
			if fires {
				want = "open"
			}
			if got := activeLeaf(gate.State); got != want {
				t.Fatalf("state after Poke = %s, want %s", got, want)
			}
			if chosen := ctx.selectedVariants[selection]; chosen != variant {
				t.Errorf("after dispatch: power selected %q, want %s", chosen, variant)
			}
		})
	}
}

// A guard reading a variation of the performer that nothing has read yet
// materializes its selection: the variant chosen for that owner, the object it
// stands for, and the feature value holding it. Decide leaves none of that on
// the performer, whether the guard lets the transition fire or refuses it, and
// the dispatch that follows selects the variant for real.
func TestDecideLeavesNoVariationSelectionAGuardMaterializes(t *testing.T) {
	src := `
		private import ScalarValues::*;
		attribute def Poke;
		part def Engine { attribute power : Real; }
		part def Car {
			variation part engine : Engine {
				variant part electric : Engine { attribute :>> power = 150.0; }
				variant part petrol : Engine { attribute :>> power = 120.0; }
			}
			exhibit state gate {
				entry; then shut;
				state shut;
				transition first shut accept Poke if engine.power > 130.0 then open;
				state open;
			}
		}
		part car : Car { part :>> engine = engine::%s; }
	`
	for variant, fires := range map[string]bool{"electric": true, "petrol": false} {
		t.Run(variant, func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "car.sysml", parseAndBuild(t, fmt.Sprintf(src, variant)))
			root := idx.DocumentRoot("car.sysml")
			car, err := ctx.occurrenceOf(resolveSymbol(t, root, "car"))
			if err != nil {
				t.Fatalf("occurrenceOf(car): %v", err)
			}
			gate, _ := car.ExhibitedState()
			poke, err := ctx.SignalMessage(resolveSymbol(t, root, "Poke"), nil, car)
			if err != nil {
				t.Fatalf("SignalMessage(Poke): %v", err)
			}
			engine := car.FeatureValues["engine"]
			if engine == nil || engine.Materialized {
				t.Fatalf("engine = %+v; want an unmaterialized feature value before anything reads it", engine)
			}
			instances, selections, variants := len(ctx.instances), len(ctx.selectedVariants), len(ctx.variantObjects)

			d, err := gate.State.Decide(poke)
			if err != nil || (len(d.Fires) == 1) != fires {
				t.Fatalf("Decide(Poke) = %+v, %v; want firing=%v", d, err, fires)
			}
			if engine.Materialized || engine.Value.Kind != ValInvalid {
				t.Errorf("engine = %+v after Decide; want it unmaterialized as before", engine)
			}
			if len(ctx.instances) != instances || len(ctx.selectedVariants) != selections || len(ctx.variantObjects) != variants {
				t.Errorf("after Decide: %d objects, %d selections, %d variant objects; want %d, %d, %d as before",
					len(ctx.instances), len(ctx.selectedVariants), len(ctx.variantObjects), instances, selections, variants)
			}
			for key, id := range ctx.variantObjects {
				if _, live := ctx.instances[id]; !live {
					t.Errorf("variant %s of %s names object #%d, which is gone", key.variant.Name, key.variation.Name, id)
				}
			}
			if got := activeLeaf(gate.State); got != "shut" {
				t.Fatalf("state after Decide = %s, want shut", got)
			}

			ctx.PostMessage(poke)
			if err := gate.State.ProcessNextEvent(); err != nil {
				t.Fatalf("ProcessNextEvent(Poke): %v", err)
			}
			want := "shut"
			if fires {
				want = "open"
			}
			if got := activeLeaf(gate.State); got != want {
				t.Fatalf("state after Poke = %s, want %s", got, want)
			}
			if !engine.Materialized || engine.Value.Kind != ValVariant || engine.Value.Variant().Name != variant {
				t.Fatalf("engine = %+v after dispatch; want the %s variant selected", engine, variant)
			}
			if _, live := ctx.instances[engine.Value.Instance]; !live {
				t.Errorf("the selected variant's object #%d is not held by the session", engine.Value.Instance)
			}
		})
	}
}

// Two machines of one object both accept Ping. The message goes to the first, in
// the order the object exhibits them, that would fire on it: one whose guard
// refuses it leaves it in flight for a sibling that would, and only takes and
// drops it when no sibling would either.
func TestSignalGoesToTheSiblingMachineThatFiresOnIt(t *testing.T) {
	src := `
		private import ScalarValues::*;
		attribute def Ping;
		part def Twin {
			attribute leftArmed : Boolean = %v;
			attribute rightArmed : Boolean = %v;
			exhibit state left {
				entry; then idle;
				state idle;
				transition first idle accept Ping if leftArmed then done;
				state done;
			}
			exhibit state right {
				entry; then idle;
				state idle;
				transition first idle accept Ping if rightArmed then done;
				state done;
			}
		}
		part twin : Twin;
	`
	cases := []struct {
		name                     string
		leftArmed, rightArmed    bool
		wantLeft, wantRight      string
		leftStepLeavesItInFlight bool
	}{
		{"only_the_second_fires", false, true, "idle", "done", true},
		{"both_fire", true, true, "done", "idle", false},
		{"neither_fires", false, false, "idle", "idle", false},
	}
	// twinWithPing builds the object, its two machines and a Ping addressed to it.
	twinWithPing := func(t *testing.T, tc struct {
		leftArmed, rightArmed bool
	}) (*Context, *StateExecutor, *StateExecutor, Message) {
		t.Helper()
		idx, _, ctx := buildRuntimeWithLibraries(t, "twin.sysml", parseAndBuild(t, fmt.Sprintf(src, tc.leftArmed, tc.rightArmed)))
		root := idx.DocumentRoot("twin.sysml")
		twin, err := ctx.occurrenceOf(resolveSymbol(t, root, "twin"))
		if err != nil {
			t.Fatalf("occurrenceOf(twin): %v", err)
		}
		var left, right *StateExecutor
		for _, b := range twin.Behaviors() {
			switch b.Name {
			case "left":
				left = b.State
			case "right":
				right = b.State
			}
		}
		if left == nil || right == nil {
			t.Fatalf("twin runs %d behaviors; want left and right", len(twin.Behaviors()))
		}
		ping, err := ctx.SignalMessage(resolveSymbol(t, root, "Ping"), nil, twin)
		if err != nil {
			t.Fatalf("SignalMessage(Ping): %v", err)
		}
		return ctx, left, right, ping
	}
	for _, tc := range cases {
		armed := struct{ leftArmed, rightArmed bool }{tc.leftArmed, tc.rightArmed}
		want := tc.wantLeft + "/" + tc.wantRight
		t.Run(tc.name+"/stepped", func(t *testing.T) {
			// Stepped on its own, the left machine takes the message only when it
			// is the one that should.
			ctx, left, right, ping := twinWithPing(t, armed)
			ctx.PostMessage(ping)
			if left.HasPendingSignal() == tc.leftStepLeavesItInFlight {
				t.Fatalf("left.HasPendingSignal() = %v; want the opposite", !tc.leftStepLeavesItInFlight)
			}
			if left.HasPendingSignal() {
				if err := left.ProcessNextEvent(); err != nil {
					t.Fatalf("left.ProcessNextEvent: %v", err)
				}
			}
			if (len(ctx.PendingMessages()) == 1) != tc.leftStepLeavesItInFlight {
				t.Fatalf("after stepping left: %d messages in flight, want left to leave it = %v", len(ctx.PendingMessages()), tc.leftStepLeavesItInFlight)
			}
			if tc.leftStepLeavesItInFlight {
				if err := right.ProcessNextEvent(); err != nil {
					t.Fatalf("right.ProcessNextEvent: %v", err)
				}
			}
			if got := activeLeaf(left) + "/" + activeLeaf(right); got != want {
				t.Fatalf("after stepping: left/right = %s, want %s", got, want)
			}
			if len(ctx.PendingMessages()) != 0 {
				t.Fatalf("%d messages left in flight after both machines stepped", len(ctx.PendingMessages()))
			}
		})
		t.Run(tc.name+"/drained", func(t *testing.T) {
			// Run collectively, the same message reaches the same machine.
			ctx, left, right, ping := twinWithPing(t, armed)
			ctx.PostMessage(ping)
			if err := ctx.drainObjectBehaviors(); err != nil {
				t.Fatalf("drainObjectBehaviors: %v", err)
			}
			if got := activeLeaf(left) + "/" + activeLeaf(right); got != want {
				t.Fatalf("after the drain: left/right = %s, want %s", got, want)
			}
			if len(ctx.PendingMessages()) != 0 {
				t.Fatalf("%d messages left in flight after the drain", len(ctx.PendingMessages()))
			}
		})
	}
}

// A guard a sibling machine cannot evaluate on the message is that sibling's
// error to report: the machine whose guard merely refuses the message leaves it
// in flight instead of consuming it, and the error surfaces where it is
// dispatched, stepped or drained alike.
func TestSignalLeftForASiblingWhoseGuardFails(t *testing.T) {
	src := `
		private import ScalarValues::*;
		attribute def Ping { attribute level : Integer; }
		part def Twin {
			attribute leftArmed : Boolean = false;
			exhibit state left {
				entry; then idle;
				state idle;
				transition first idle accept Ping if leftArmed then done;
				state done;
			}
			exhibit state right {
				entry; then idle;
				state idle;
				transition right_go first idle accept p : Ping if 10 / p.level > 1 then done;
				state done;
			}
		}
		part twin : Twin;
	`
	twinWithPing := func(t *testing.T) (*Context, *StateExecutor, *StateExecutor, Message) {
		t.Helper()
		idx, _, ctx := buildRuntimeWithLibraries(t, "twin.sysml", parseAndBuild(t, src))
		root := idx.DocumentRoot("twin.sysml")
		twin, err := ctx.occurrenceOf(resolveSymbol(t, root, "twin"))
		if err != nil {
			t.Fatalf("occurrenceOf(twin): %v", err)
		}
		var left, right *StateExecutor
		for _, b := range twin.Behaviors() {
			switch b.Name {
			case "left":
				left = b.State
			case "right":
				right = b.State
			}
		}
		if left == nil || right == nil {
			t.Fatalf("twin runs %d behaviors; want left and right", len(twin.Behaviors()))
		}
		ping, err := ctx.SignalMessage(resolveSymbol(t, root, "Ping"), map[string]Value{"level": integerValue(0)}, twin)
		if err != nil {
			t.Fatalf("SignalMessage(Ping): %v", err)
		}
		return ctx, left, right, ping
	}
	const wantErr = "eval guard of transition right_go: division by zero"

	t.Run("stepped", func(t *testing.T) {
		ctx, left, right, ping := twinWithPing(t)
		ctx.PostMessage(ping)
		if left.HasPendingSignal() {
			t.Fatal("the left machine, whose guard refuses Ping, took it from the sibling whose guard fails on it")
		}
		if !right.HasPendingSignal() {
			t.Fatal("the right machine, whose guard fails on Ping, does not have it pending")
		}
		if got := len(ctx.PendingMessages()); got != 1 {
			t.Fatalf("%d messages in flight before the right machine steps, want 1", got)
		}
		err := right.ProcessNextEvent()
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("right.ProcessNextEvent() = %v, want the guard error %q", err, wantErr)
		}
		if got := activeLeaf(left) + "/" + activeLeaf(right); got != "idle/idle" {
			t.Errorf("left/right = %s after the failed guard, want idle/idle", got)
		}
	})
	t.Run("drained", func(t *testing.T) {
		ctx, left, right, ping := twinWithPing(t)
		ctx.PostMessage(ping)
		err := ctx.drainObjectBehaviors()
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Fatalf("drainObjectBehaviors() = %v, want the guard error %q", err, wantErr)
		}
		if got := activeLeaf(left) + "/" + activeLeaf(right); got != "idle/idle" {
			t.Errorf("left/right = %s after the failed guard, want idle/idle", got)
		}
	})
}

// A derived value a guard materializes under Decide is dropped with the probe,
// so dispatch after a message rewrote its dependency derives it afresh.
func TestDecideLeavesNoValueAGuardMaterializes(t *testing.T) {
	src := `
		private import ScalarValues::*;
		attribute def Bump;
		attribute def Go;
		part def Dimmer {
			attribute base : Integer = 1;
			attribute threshold : Integer = base * 2;
			exhibit state life {
				entry; then idle;
				state idle;
				transition first idle accept Bump do assign base := 5 then idle;
				transition idle_on first idle accept Go if threshold > 5 then on;
				state on;
			}
		}
		part dimmer : Dimmer;
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "dimmer.sysml", parseAndBuild(t, src))
	root := idx.DocumentRoot("dimmer.sysml")
	dimmer, err := ctx.occurrenceOf(resolveSymbol(t, root, "dimmer"))
	if err != nil {
		t.Fatalf("occurrenceOf(dimmer): %v", err)
	}
	life, ok := dimmer.ExhibitedState()
	if !ok {
		t.Fatal("the dimmer exhibits no machine")
	}
	bump, err := ctx.SignalMessage(resolveSymbol(t, root, "Bump"), nil, dimmer)
	if err != nil {
		t.Fatalf("SignalMessage(Bump): %v", err)
	}
	goMsg, err := ctx.SignalMessage(resolveSymbol(t, root, "Go"), nil, dimmer)
	if err != nil {
		t.Fatalf("SignalMessage(Go): %v", err)
	}
	threshold := dimmer.FeatureValues["threshold"]
	if threshold == nil || threshold.Materialized {
		t.Fatalf("threshold = %+v; want an unmaterialized feature value before anything reads it", threshold)
	}

	ctx.PostMessage(bump)
	if d, err := life.State.Decide(goMsg); err != nil || d.Enabled() {
		t.Fatalf("Decide(Go) with base 1 = %+v, %v; want nothing enabled", d, err)
	}
	if threshold.Materialized || threshold.Value.Kind != ValInvalid {
		t.Errorf("threshold = %+v after Decide; want it unmaterialized as before", threshold)
	}
	ctx.PostMessage(goMsg)

	if err := life.State.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Bump): %v", err)
	}
	if got := FormatValue(dimmer.FeatureValues["base"].HeldValue()); got != "5" {
		t.Fatalf("base = %s after Bump, want 5", got)
	}
	if d, err := life.State.Decide(goMsg); err != nil || len(d.Fires) != 1 || d.Fires[0] != "transition idle_on" {
		t.Errorf("Decide(Go) with base 5 = %+v, %v; want idle_on firing", d, err)
	}
	if err := life.State.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Go): %v", err)
	}
	if got := activeLeaf(life.State); got != "on" {
		t.Fatalf("state after Go = %s, want on: the guard read the threshold the probe derived from base 1", got)
	}
	if got := FormatValue(threshold.HeldValue()); got != "10" {
		t.Errorf("threshold = %s after dispatch, want 10 derived from base 5", got)
	}
}

// An object a guard materializes starts its behaviors under Decide as under
// dispatch (its entry sets level to 10), and is gone with them after the decision.
// The sensor is a package-level occurrence, which the controller's creation
// leaves to the guard.
func TestDecideStartsTheBehaviorsOfAnObjectAGuardMaterializes(t *testing.T) {
	src := `
		private import ScalarValues::*;
		attribute def Go;
		attribute def Halt;
		part def Sensor {
			attribute level : Integer = 1;
			exhibit state calibrate {
				entry; then ready;
				state ready { entry assign level := 10; }
			}
		}
		part sensor : Sensor;
		part def Controller {
			exhibit state life {
				entry; then off;
				state off;
				transition off_on first off accept Go if sensor.level > 5 then on;
				transition off_halt first off accept Halt if sensor.level < 5 then halted;
				state on;
				state halted;
			}
		}
		part ctl : Controller;
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "ctl.sysml", parseAndBuild(t, src))
	root := idx.DocumentRoot("ctl.sysml")
	ctl, err := ctx.occurrenceOf(resolveSymbol(t, root, "ctl"))
	if err != nil {
		t.Fatalf("occurrenceOf(ctl): %v", err)
	}
	life, ok := ctl.ExhibitedState()
	if !ok {
		t.Fatal("the controller exhibits no machine")
	}
	goMsg, err := ctx.SignalMessage(resolveSymbol(t, root, "Go"), nil, ctl)
	if err != nil {
		t.Fatalf("SignalMessage(Go): %v", err)
	}
	halt, err := ctx.SignalMessage(resolveSymbol(t, root, "Halt"), nil, ctl)
	if err != nil {
		t.Fatalf("SignalMessage(Halt): %v", err)
	}
	sensor := resolveSymbol(t, root, "sensor")
	if _, built := ctx.occurrences[sensor]; built {
		t.Fatal("sensor is built before anything reads it")
	}
	instances, behaviors, messages := len(ctx.instances), len(ctx.objectBehaviors), len(ctx.messages)

	if d, err := life.State.Decide(goMsg); err != nil || len(d.Fires) != 1 || d.Fires[0] != "transition off_on" {
		t.Errorf("Decide(Go) = %+v, %v; want off_on firing on the level the sensor's machine set", d, err)
	}
	if d, err := life.State.Decide(halt); err != nil || d.Enabled() {
		t.Errorf("Decide(Halt) = %+v, %v; want nothing enabled on the level the sensor's machine set", d, err)
	}
	if _, built := ctx.occurrences[sensor]; built {
		t.Error("the sensor the guards read outlives the decisions")
	}
	if len(ctx.instances) != instances || len(ctx.objectBehaviors) != behaviors || len(ctx.messages) != messages {
		t.Errorf("after Decide: %d objects, %d behaviors, %d messages; want %d, %d, %d as before",
			len(ctx.instances), len(ctx.objectBehaviors), len(ctx.messages), instances, behaviors, messages)
	}
	if got := activeLeaf(life.State); got != "off" {
		t.Fatalf("state after Decide = %s, want off", got)
	}

	ctx.PostMessage(goMsg)
	if err := life.State.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Go): %v", err)
	}
	if got := activeLeaf(life.State); got != "on" {
		t.Fatalf("state after Go = %s, want on", got)
	}
	id, ok := ctx.occurrences[sensor]
	if !ok {
		t.Fatal("dispatching Go did not build the sensor its guard reads")
	}
	built := ctx.instances[id]
	if got := FormatValue(built.FeatureValues["level"].HeldValue()); got != "10" {
		t.Errorf("sensor.level = %s after dispatch, want 10 set by its machine", got)
	}
	if calibrate, ok := built.ExhibitedState(); !ok || activeLeaf(calibrate.State) != "ready" {
		t.Errorf("the built sensor runs no calibrate machine at ready: ok=%v", ok)
	}
}

// A drain under a probe runs only the behaviors the probe attached: a startup run
// still pending from the collective start under way — a sibling machine's, another
// object's action — is left to that start, in place, neither advanced nor dropped.
func TestProbeDrainLeavesThePendingStartupRunsOfTheStartUnderWay(t *testing.T) {
	src := `
		private import ScalarValues::*;
		attribute def Ping;
		part def Sensor {
			attribute level : Integer = 1;
			exhibit state calibrate {
				entry; then ready;
				state ready { entry assign level := 10; }
			}
		}
		part def Twin {
			part sensor : Sensor;
			exhibit state left {
				entry; then idle;
				state idle;
				transition left_go first idle accept Ping if sensor.level > 5 then done;
				state done;
			}
			exhibit state right {
				entry; then idle;
				state idle;
				transition first idle accept Ping then done;
				state done;
			}
		}
		part def Pump {
			attribute primed : Boolean = false;
			perform action prime {
				action step { assign primed := true; }
				first step;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "twin.sysml", parseAndBuild(t, src))
	root := idx.DocumentRoot("twin.sysml")
	twin, err := ctx.materialize(resolveSymbol(t, root, "Twin"), 0, nil, "")
	if err != nil {
		t.Fatalf("materialize(Twin): %v", err)
	}
	pump, err := ctx.materialize(resolveSymbol(t, root, "Pump"), 0, nil, "")
	if err != nil {
		t.Fatalf("materialize(Pump): %v", err)
	}
	// The attach phase of a collective start of both objects, before its run.
	ctx.behaviorRunDepth++
	for _, obj := range []*Instance{twin, pump} {
		if err := ctx.startBehaviorsOf(obj); err != nil {
			t.Fatalf("startBehaviorsOf(%s): %v", symbolText(obj.Type), err)
		}
	}
	ctx.behaviorRunDepth--
	pending := append([]*ObjectBehavior(nil), ctx.pendingBehaviors...)
	if len(pending) != 3 {
		t.Fatalf("%d startup runs pending, want left, right and prime", len(pending))
	}
	left, right, prime := pending[0], pending[1], pending[2]
	if left.Name != "left" || right.Name != "right" || prime.Name != "prime" {
		t.Fatalf("pending %s, %s, %s; want left, right, prime", left.Name, right.Name, prime.Name)
	}
	ping, err := ctx.SignalMessage(resolveSymbol(t, root, "Ping"), nil, twin)
	if err != nil {
		t.Fatalf("SignalMessage(Ping): %v", err)
	}
	sensor := twin.FeatureValues["sensor"]
	if sensor == nil || sensor.Materialized {
		t.Fatalf("sensor = %+v; want an unmaterialized feature value before the guard reads it", sensor)
	}

	// The left machine's preflight of Ping, as its dispatch of a message in flight
	// or %send makes: the guard materializes the sensor, whose start drains.
	if d, err := left.State.Decide(ping); err != nil || len(d.Fires) != 1 || d.Fires[0] != "transition left_go" {
		t.Errorf("Decide(Ping) = %+v, %v; want left_go firing on the level the sensor's machine set", d, err)
	}
	if got := ctx.pendingBehaviors; len(got) != 3 || got[0] != left || got[1] != right || got[2] != prime {
		t.Errorf("after Decide, %d startup runs pending; want left, right and prime as before", len(got))
	}
	if got := activeLeaf(right.State); got != "idle" {
		t.Errorf("the right machine is at %s after the left one's Decide, want idle", got)
	}
	if prime.Action.State() == StateCompleted {
		t.Error("the pump's prime ran under the left machine's Decide")
	}
	if sensor.Materialized || len(twin.Behaviors()) != 2 || len(pump.Behaviors()) != 1 {
		t.Errorf("after Decide: sensor %+v, twin runs %d behaviors, pump %d; want it unmaterialized, 2 and 1",
			sensor, len(twin.Behaviors()), len(pump.Behaviors()))
	}

	// The start's own run then takes every pending startup run in order.
	if err := ctx.runAttachedBehaviors(); err != nil {
		t.Fatalf("runAttachedBehaviors: %v", err)
	}
	if len(ctx.pendingBehaviors) != 0 {
		t.Errorf("%d startup runs still pending after the start ran", len(ctx.pendingBehaviors))
	}
	if got := activeLeaf(left.State) + "/" + activeLeaf(right.State); got != "idle/idle" {
		t.Errorf("left/right = %s after the start ran, want idle/idle", got)
	}
	if got := FormatValue(pump.FeatureValues["primed"].HeldValue()); got != "true" || prime.Action.State() != StateCompleted {
		t.Errorf("primed = %s with prime %s after the start ran; want true and the action completed", got, prime.Action.State())
	}
}

// A start does not dispatch a message a driver left in flight for a machine that
// was already quiescent: the driver stepping the machine dispatches it, so a
// signal accepted at send time and undercut by a later one is still reported by
// the step that consumes it, not swallowed by an unrelated object's creation.
func TestStartLeavesAMessageInFlightToTheDriverSteppingItsMachine(t *testing.T) {
	src := `
		private import ScalarValues::*;
		attribute def Poke;
		attribute def Lock;
		part def Keeper {
			attribute open : Boolean = true;
			exhibit state gate {
				entry; then shut;
				state shut;
				transition lock first shut accept Lock do assign open := false then shut;
				transition shut_through first shut accept Poke if open then through;
				state through;
			}
		}
		part def Bystander {
			exhibit state life { entry; then alive; state alive; }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "keeper.sysml", parseAndBuild(t, src))
	root := idx.DocumentRoot("keeper.sysml")
	keeper, err := ctx.Instantiate(resolveSymbol(t, root, "Keeper"))
	if err != nil {
		t.Fatalf("Instantiate(Keeper): %v", err)
	}
	gate, ok := keeper.ExhibitedState()
	if !ok || activeLeaf(gate.State) != "shut" {
		t.Fatalf("keeper runs no gate machine at shut: ok=%v", ok)
	}
	for _, name := range []string{"Lock", "Poke"} {
		msg, err := ctx.SignalMessage(resolveSymbol(t, root, name), nil, keeper)
		if err != nil {
			t.Fatalf("SignalMessage(%s): %v", name, err)
		}
		ctx.PostMessage(msg)
	}

	// An unrelated object's start, as a driver's read or command may make between
	// dispatching two messages it has in flight.
	if _, err := ctx.Instantiate(resolveSymbol(t, root, "Bystander")); err != nil {
		t.Fatalf("Instantiate(Bystander): %v", err)
	}
	if got := len(ctx.PendingMessages()); got != 2 {
		t.Fatalf("%d messages in flight after the bystander's start, want Lock and Poke left for the driver", got)
	}
	if got := FormatValue(keeper.FeatureValues["open"].HeldValue()); got != "true" {
		t.Errorf("open = %s after the bystander's start, want true: Lock was not the start's to dispatch", got)
	}

	// The driver then dispatches each in turn: Lock shuts the gate, so Poke is
	// consumed by no transition.
	if err := gate.State.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Lock): %v", err)
	}
	if got := FormatValue(keeper.FeatureValues["open"].HeldValue()); got != "false" {
		t.Errorf("open = %s after Lock, want false", got)
	}
	if got := len(ctx.PendingMessages()); got != 1 {
		t.Fatalf("%d messages in flight after Lock, want Poke", got)
	}
	if err := gate.State.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Poke): %v", err)
	}
	d, ok := gate.State.LastDispatch()
	if !ok || d.Fired || d.Deferred {
		t.Errorf("last dispatch = %+v, %v; want Poke dispatched, firing nothing", d, ok)
	}
	if msg, isSignal := d.Event.Payload.(Message); !isSignal || msg.SignalType != "Poke" {
		t.Errorf("last dispatch carried %+v, want the Poke message", d.Event.Payload)
	}
	if got := activeLeaf(gate.State); got != "shut" || len(ctx.PendingMessages()) != 0 {
		t.Errorf("gate at %s with %d messages in flight, want shut and none", got, len(ctx.PendingMessages()))
	}
}

// A message in flight is left as Decide found it, payload included: the machine
// of an object a guard materializes may take it under the probe and cache the
// occurrence it binds on the payload, an object the probe discards. Dispatch
// then binds a live occurrence. The sensor is a package-level occurrence, which
// the controller's creation leaves to the guard.
func TestDecideLeavesNoOccurrenceCachedOnAMessageInFlight(t *testing.T) {
	src := `
		private import ScalarValues::*;
		attribute def Go;
		attribute def Kick { attribute n : Integer; }
		part def Sensor {
			attribute level : Integer = 1;
			exhibit state calibrate {
				entry; then ready;
				state ready { entry assign level := 10; }
				transition first ready accept k : Kick then kicked;
				state kicked;
			}
		}
		part sensor : Sensor;
		part def Controller {
			exhibit state life {
				entry; then off;
				state off;
				transition off_on first off accept Go if sensor.level > 5 then on;
				state on;
			}
		}
		part ctl : Controller;
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "ctl.sysml", parseAndBuild(t, src))
	root := idx.DocumentRoot("ctl.sysml")
	ctl, err := ctx.occurrenceOf(resolveSymbol(t, root, "ctl"))
	if err != nil {
		t.Fatalf("occurrenceOf(ctl): %v", err)
	}
	life, ok := ctl.ExhibitedState()
	if !ok {
		t.Fatal("the controller exhibits no machine")
	}
	kick, err := ctx.SignalMessage(resolveSymbol(t, root, "Kick"), map[string]Value{"n": integerValue(3)}, nil)
	if err != nil {
		t.Fatalf("SignalMessage(Kick): %v", err)
	}
	ctx.PostMessage(kick)
	goMsg, err := ctx.SignalMessage(resolveSymbol(t, root, "Go"), nil, ctl)
	if err != nil {
		t.Fatalf("SignalMessage(Go): %v", err)
	}

	if d, err := life.State.Decide(goMsg); err != nil || len(d.Fires) != 1 || d.Fires[0] != "transition off_on" {
		t.Errorf("Decide(Go) = %+v, %v; want off_on firing on the level the sensor's machine set", d, err)
	}
	pending := ctx.PendingMessages()
	if len(pending) != 1 || pending[0].SignalType != "Kick" {
		t.Fatalf("after Decide, on the bus: %+v; want the Kick still in flight", pending)
	}
	if pending[0].Value != nil || len(pending[0].Payload) != 1 {
		t.Errorf("Kick after Decide = %+v; want the argument alone, no occurrence cached by the probe", pending[0])
	}

	ctx.PostMessage(goMsg)
	if err := life.State.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Go): %v", err)
	}
	id, built := ctx.occurrences[resolveSymbol(t, root, "sensor")]
	if got := activeLeaf(life.State); got != "on" || !built {
		t.Fatalf("after Go: state %s, sensor built %v; want on with the sensor built", got, built)
	}
	calibrate, ok := ctx.instances[id].ExhibitedState()
	if !ok || activeLeaf(calibrate.State) != "kicked" {
		t.Fatalf("the built sensor's machine is not at kicked: ok=%v", ok)
	}
	k := calibrate.State.StateData()["k"]
	if k.Kind != ValInstance {
		t.Fatalf("k = %+v after Kick, want the occurrence it binds", k)
	}
	if _, live := ctx.instances[k.Instance]; !live {
		t.Errorf("k names object #%d, which the session does not hold: the probe's occurrence was bound", k.Instance)
	}
}

// Whether a message routed to a port by identity reaches the machine is told by
// materializing the port the accept names; a preview builds it and lets it go,
// as it does everything else, so the object is as it was found. Dispatch
// materializes the port for real.
func TestPreviewsDiscardThePortTheyMaterialize(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;`+directedPorts+`
		part def Listener {
			port in : ~Chan;
			exhibit state sm {
				entry; then Idle;
				state Idle { accept v : Integer via in then Got; }
				state Got;
			}
		}
		part listener : Listener;
	}`))
	listener, err := ctx.Instantiate(oneSymbol(t, idx, "P::listener"))
	if err != nil {
		t.Fatalf("instantiate listener: %v", err)
	}
	exec, err := ctx.CreateStateExecutorFor(oneSymbol(t, idx, "P::Listener::sm"), listener)
	if err != nil {
		t.Fatalf("create state executor: %v", err)
	}
	in := listener.FeatureValues["in"]
	if in == nil || in.Materialized {
		t.Fatalf("in = %+v; want an unmaterialized port before anything reads it", in)
	}
	instances, behaviors := len(ctx.instances), len(ctx.objectBehaviors)
	// Delivered to a port object by identity, under a name the accept does not
	// use, so only materializing `in` can tell whether it is the same port.
	four := integerValue(4)
	elsewhere := Message{SignalType: "Integer", Port: "other", Object: listener.ID, PortID: -1,
		Delivery: DeliverPort, Value: &four}
	if accepted, err := exec.AcceptsMessage(elsewhere); err != nil || accepted {
		t.Errorf("AcceptsMessage(Integer at another port) = %v, %v; want it left", accepted, err)
	}
	if in.Materialized || len(ctx.instances) != instances || len(ctx.objectBehaviors) != behaviors {
		t.Errorf("after AcceptsMessage: in %+v, %d objects, %d behaviors; want the port unmaterialized, %d, %d as before",
			in, len(ctx.instances), len(ctx.objectBehaviors), instances, behaviors)
	}
	if d, err := exec.Decide(elsewhere); err != nil || d.Enabled() {
		t.Errorf("Decide(Integer at another port) = %+v, %v; want nothing enabled", d, err)
	}
	if in.Materialized || len(ctx.instances) != instances || len(ctx.objectBehaviors) != behaviors {
		t.Errorf("after Decide: in %+v, %d objects, %d behaviors; want the port unmaterialized, %d, %d as before",
			in, len(ctx.instances), len(ctx.objectBehaviors), instances, behaviors)
	}

	ctx.PostMessage(elsewhere)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run with a message for another port in flight: %v", err)
	}
	if !in.Materialized || in.Value.Kind != ValInstance || len(ctx.instances) != instances+1 {
		t.Fatalf("after the run: in %+v, %d objects; want the port built to tell the message is not for it", in, len(ctx.instances))
	}
	if got := activeLeaf(exec); got != "Idle" || len(ctx.PendingMessages()) != 1 {
		t.Fatalf("after the run: state %s, %d in flight; want Idle with the message left", got, len(ctx.PendingMessages()))
	}
	atIn := elsewhere
	atIn.PortID = in.Value.Instance
	if d, err := exec.Decide(atIn); err != nil || len(d.Fires) != 1 {
		t.Errorf("Decide(Integer at in's object) = %+v, %v; want the accept via in firing", d, err)
	}
	ctx.PostMessage(atIn)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run with the message at in: %v", err)
	}
	if got := activeLeaf(exec); got != "Got" {
		t.Errorf("state after the message at in = %s, want Got", got)
	}
}

const adoptLinkedSrc = `package Demo {
	private import ScalarValues::*;
	attribute def Go;
	port def P { attribute rate : Real = 3.0; }
	part def A { port p : P; }
	part def B { port q : P; }
	connection def Link { end source : P; end target : P; }
	part def Sys {
		part a : A;
		part b : B;
		connection link : Link connect a.p to b.q;
		connect a.p to b.q;
		exhibit state life {
			entry; then off;
			state off;
			transition off_on first off accept Go if link.source.rate > 1.0 then on;
			state on;
		}
	}
}`

// A carried object keeps the identities of its connectors for the ones
// materialized again against the new declarations. A preview that materializes
// one on the way discards the object, so it leaves the identity kept: dispatch
// materializes the same connector, not a new one.
func TestPreviewsLeaveACarriedObjectsConnectorIdentitiesKept(t *testing.T) {
	prev := contextOver(t, adoptLinkedSrc)
	obj, err := prev.Instantiate(lookupOne(t, prev.resolver.Index(), "Demo::Sys"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	linkID := fvInstance(t, prev, obj, "link").ID
	conns, err := obj.OwnedConnectors(prev)
	if err != nil || len(conns) != 1 {
		t.Fatalf("OwnedConnectors = %v, %v; want the one anonymous connector", conns, err)
	}
	anonymousID := conns[0].ID
	shapes := prev.ShapesOf(obj)

	ctx := contextOver(t, adoptLinkedSrc+"\npart def Widget;")
	if _, err := ctx.Adopt(prev, shapes, obj); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	link := obj.FeatureValues["link"]
	if link.Materialized || obj.keptConnectors[link] != linkID {
		t.Fatalf("after the carry-over: link %+v keeping identity %d; want it unmaterialized keeping %d", link, obj.keptConnectors[link], linkID)
	}
	life, ok := obj.ExhibitedState()
	if !ok {
		t.Fatal("the carried object runs no machine")
	}
	goMsg, err := ctx.SignalMessage(lookupOne(t, ctx.resolver.Index(), "Demo::Go"), nil, obj)
	if err != nil {
		t.Fatalf("SignalMessage(Go): %v", err)
	}
	instances := len(ctx.instances)

	// The guard reads through the named connector, materializing it under the probe.
	if d, err := life.State.Decide(goMsg); err != nil || len(d.Fires) != 1 || d.Fires[0] != "transition off_on" {
		t.Errorf("Decide(Go) = %+v, %v; want off_on firing on the rate read through link", d, err)
	}
	if link.Materialized || len(ctx.instances) != instances {
		t.Errorf("after Decide: link %+v, %d objects; want it unmaterialized and %d objects as before", link, len(ctx.instances), instances)
	}
	if got := obj.keptConnectors[link]; got != linkID {
		t.Errorf("after Decide, link keeps identity %d; want %d as before", got, linkID)
	}
	// The anonymous connectors are materialized under a probe the same way.
	end := ctx.beginProbe()
	mark := len(ctx.created)
	if conns, err := obj.OwnedConnectors(ctx); err != nil || len(conns) != 1 || conns[0].ID != anonymousID {
		t.Errorf("OwnedConnectors under the probe = %v, %v; want the connector under identity %d", conns, err, anonymousID)
	}
	ctx.abandonInstancesSince(mark)
	end()
	if obj.anonymous != nil || len(obj.keptAnonymous) != 1 || obj.keptAnonymous[0].id != anonymousID {
		t.Errorf("after the probe: anonymous %v keeping %v; want none materialized keeping %d", obj.anonymous, obj.keptAnonymous, anonymousID)
	}

	ctx.PostMessage(goMsg)
	if err := life.State.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(Go): %v", err)
	}
	if got := activeLeaf(life.State); got != "on" {
		t.Fatalf("state after Go = %s, want on", got)
	}
	if !link.Materialized || !holdsObject(link.Value, linkID) {
		t.Errorf("after dispatch, link = %+v; want the connector materialized under identity %d", link, linkID)
	}
	if _, live := ctx.Instance(linkID); !live {
		t.Errorf("the context does not hold connector %d", linkID)
	}
	if conns, err := obj.OwnedConnectors(ctx); err != nil || len(conns) != 1 || conns[0].ID != anonymousID {
		t.Errorf("OwnedConnectors after the probe = %v, %v; want the connector under identity %d", conns, err, anonymousID)
	}
}

// ExhibitedStatesOf finds the machine an object runs by its definition or its
// usage, and none on an object exhibiting no such machine.
func TestExhibitedStatesOf(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "lamp.sysml", parseAndBuild(t, lampSource))
	root := idx.DocumentRoot("lamp.sysml")
	bulb, err := ctx.Instantiate(resolveSymbol(t, root, "Bulb"))
	if err != nil {
		t.Fatalf("Instantiate Bulb: %v", err)
	}
	plain, err := ctx.Instantiate(resolveSymbol(t, root, "Plain"))
	if err != nil {
		t.Fatalf("Instantiate Plain: %v", err)
	}
	lampDef := resolveSymbol(t, root, "Lamp")

	byDef := bulb.ExhibitedStatesOf(lampDef)
	if len(byDef) != 1 || byDef[0].Name != "lamp" {
		t.Fatalf("ExhibitedStatesOf(Lamp) = %v; want the lamp usage", byDef)
	}
	if byUsage := bulb.ExhibitedStatesOf(byDef[0].Member()); len(byUsage) != 1 || byUsage[0] != byDef[0] {
		t.Errorf("ExhibitedStatesOf(Bulb::lamp) = %v; want the same machine", byUsage)
	}
	if ms := plain.ExhibitedStatesOf(lampDef); len(ms) != 0 {
		t.Errorf("ExhibitedStatesOf(Lamp) on an object exhibiting none = %v", ms)
	}
	if ms := bulb.ExhibitedStatesOf(nil); len(ms) != 0 {
		t.Errorf("ExhibitedStatesOf(nil) = %v; want none", ms)
	}
}

// activeLeaf names the innermost active state.
func activeLeaf(exec *StateExecutor) string {
	states := exec.ActiveStates()
	if len(states) == 0 {
		return ""
	}
	return states[len(states)-1].Name
}
