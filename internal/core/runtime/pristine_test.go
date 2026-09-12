package runtime

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// refusal checks Pristine refuses the object with a HeldStateError giving reason.
func refusal(t *testing.T, ctx *Context, obj *Instance, what, reason string) {
	t.Helper()
	var hse *HeldStateError
	err := ctx.Pristine(obj)
	if !errors.As(err, &hse) || !strings.Contains(err.Error(), reason) {
		t.Errorf("Pristine %s = %v, want a HeldStateError saying %q", what, err, reason)
	}
}

// movedRefusal checks Pristine refuses the object for a behavior that has moved.
func movedRefusal(t *testing.T, ctx *Context, obj *Instance, what string) {
	t.Helper()
	refusal(t, ctx, obj, what, "has moved")
}

// A machine as its start left it is pristine; one that dispatched an event has moved,
// and stays so until a snapshot taken before the move is restored, which restores the
// record with the state; a snapshot taken after the move restores it moved.
func TestPristineFollowsAStateMachinesMoves(t *testing.T) {
	root, ctx, bulb := lampBulb(t)
	if err := ctx.Pristine(bulb); err != nil {
		t.Fatalf("Pristine(fresh bulb) = %v, want admitted", err)
	}
	fresh, err := ctx.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	dispatchTo(t, root, ctx, bulb, "go", nil)
	movedRefusal(t, ctx, bulb, "after a transition fired")
	moved, err := ctx.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot after the move: %v", err)
	}
	dispatchTo(t, root, ctx, bulb, "Dim", map[string]Value{"level": integerValue(7)})
	moved.Restore()
	if got := lampLeaf(t, bulb); got != "on" {
		t.Fatalf("state after restoring the moved snapshot = %s, want on", got)
	}
	movedRefusal(t, ctx, bulb, "after restoring the moved snapshot")

	fresh.Restore()
	if got := lampLeaf(t, bulb); got != "off" {
		t.Fatalf("state after restoring the fresh snapshot = %s, want off", got)
	}
	if err := ctx.Pristine(bulb); err != nil {
		t.Errorf("Pristine after restoring the fresh snapshot = %v, want admitted", err)
	}

	// A signal posted to the object and not yet dispatched is not a move of the
	// machine, but a fresh object would not receive it: refused for that reason.
	msg, err := ctx.SignalMessage(resolveSymbol(t, root, "go"), nil, bulb)
	if err != nil {
		t.Fatalf("SignalMessage: %v", err)
	}
	ctx.PostMessage(msg)
	refusal(t, ctx, bulb, "with a signal posted to it", "has a go posted to it awaiting dispatch")
}

// A performed action parked at the accept its start reached is pristine; consuming
// the accept and writing a feature from the body moves it, and the record follows
// the action through a snapshot as the machine's does.
func TestPristineFollowsAPerformedActionsMoves(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, waiterSource)
	ctx := NewContext(NewModel(model, resolver), 10000)
	waiter, err := ctx.Instantiate(resolveSymbol(t, resolveSymbol(t, root, "test").Scope, "Waiter"))
	if err != nil {
		t.Fatalf("Instantiate Waiter: %v", err)
	}
	if err := ctx.Pristine(waiter); err != nil {
		t.Fatalf("Pristine(waiter parked by its start) = %v, want admitted", err)
	}
	parked, err := ctx.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	behavior, _ := waiter.Behavior("await")
	nine := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 9}}
	ctx.PostMessage(Message{SignalType: "Integer", Object: waiter.ID, Value: &nine})
	if err := behavior.Action.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	if got := featureInt(t, ctx, waiter, "woken"); got != 9 {
		t.Fatalf("woken = %d after the message, want 9", got)
	}
	movedRefusal(t, ctx, waiter, "after the accept was taken and the body wrote")

	parked.Restore()
	if got := featureInt(t, ctx, waiter, "woken"); got != 0 {
		t.Fatalf("woken = %d after restoring the parked snapshot, want 0", got)
	}
	if err := ctx.Pristine(waiter); err != nil {
		t.Errorf("Pristine after restoring the parked snapshot = %v, want admitted", err)
	}
}

const fuseSource = `
	private import SI::*;
	state def Fuse {
		entry; then armed;
		state armed;
		transition burn first armed accept after 2 [s] then spent;
		state spent;
	}
	part def Charge { exhibit state fuse : Fuse; }
	part def Plain;
`

// A timer set by the start is pristine while the clock stands at zero, where a
// fresh context's clock would set it too. Once the clock has moved, the wait is
// due at another instant than a fresh declaration's: refused though nothing fired,
// and the image carries the wait as it stands, so the copy fires when the object would.
func TestPristineRefusesATimedBehaviorOnceTheClockHasMoved(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "fuse.sysml", parseAndBuild(t, fuseSource))
	root := idx.DocumentRoot("fuse.sysml")
	charge, err := ctx.Instantiate(resolveSymbol(t, root, "Charge"))
	if err != nil {
		t.Fatalf("Instantiate Charge: %v", err)
	}
	if err := ctx.Pristine(charge); err != nil {
		t.Fatalf("Pristine(fresh charge) = %v, want admitted", err)
	}

	report, err := ctx.Advance(1)
	if err != nil {
		t.Fatalf("Advance(1): %v", err)
	}
	if report.Events != 0 || lampLeaf(t, charge) != "armed" {
		t.Fatalf("Advance(1) dispatched %d events, leaf %s; want nothing due yet", report.Events, lampLeaf(t, charge))
	}
	fuse, _ := charge.ExhibitedState()
	if fuse.Moved() {
		t.Fatal("the machine reads as moved though no event was dispatched")
	}
	refusal(t, ctx, charge, "with the clock at 1", "waits on the clock (time -> spent) with the clock at t=1.0, not at zero")

	// Only an execution waiting on the clock cares where the clock stands.
	plain, err := ctx.Instantiate(resolveSymbol(t, root, "Plain"))
	if err != nil {
		t.Fatalf("Instantiate Plain: %v", err)
	}
	if err := ctx.Pristine(plain); err != nil {
		t.Errorf("Pristine(plain object at t=1) = %v, want admitted", err)
	}

	// The copy's wait is due at t=2 as the object's is; a fresh declaration's would
	// be due 2 s after its own clock's zero.
	dst := imageInto(t, ctx, charge)
	copied, _ := dst.Instance(charge.ID)
	fresh := NewContext(ctx.Model(), 10000)
	declared, err := fresh.Instantiate(resolveSymbol(t, root, "Charge"))
	if err != nil {
		t.Fatalf("Instantiate Charge afresh: %v", err)
	}
	for name, c := range map[string]*Context{"copy": dst, "fresh": fresh} {
		if _, err := c.Advance(1); err != nil {
			t.Fatalf("Advance(%s, 1): %v", name, err)
		}
	}
	if got := lampLeaf(t, copied); got != "spent" {
		t.Errorf("the copy is at %s after 1 s more, want spent", got)
	}
	if got := lampLeaf(t, declared); got != "armed" {
		t.Errorf("a fresh declaration is at %s after 1 s, want armed", got)
	}
	if got := lampLeaf(t, charge); got != "armed" {
		t.Errorf("the source moved to %s with the copy's clock, want armed", got)
	}
}

// A message addressed to no object in particular is open to any execution that
// accepts it: a behaving object is refused while one is in flight, an object
// running nothing is not, and the image carries the message to the copy.
func TestPristineRefusesABehavingObjectWhileAnOpenMessageIsInFlight(t *testing.T) {
	root, ctx, bulb := lampBulb(t)
	plain, err := ctx.Instantiate(resolveSymbol(t, root, "Plain"))
	if err != nil {
		t.Fatalf("Instantiate Plain: %v", err)
	}
	ctx.PostMessage(NamedSignalMessage("go", nil))
	refusal(t, ctx, bulb, "with an open go in flight", "runs exhibited state machine lamp while a go addressed to no object in particular awaits dispatch")
	if err := ctx.Pristine(plain); err != nil {
		t.Errorf("Pristine(plain object) = %v, want admitted: it runs nothing the message could reach", err)
	}

	dst := imageInto(t, ctx, bulb)
	copied, _ := dst.Instance(bulb.ID)
	behavior, _ := copied.ExhibitedState()
	if err := behavior.State.ProcessNextEvent(); err != nil {
		t.Fatalf("ProcessNextEvent(copy): %v", err)
	}
	if got := lampLeaf(t, copied); got != "on" {
		t.Errorf("the copy is at %s after taking the open go, want on", got)
	}
	if got := lampLeaf(t, bulb); got != "off" {
		t.Errorf("the source is at %s, want off: the copy took its own message", got)
	}
	if n := len(ctx.PendingMessages()); n != 1 {
		t.Errorf("%d messages in flight in the source, want the open go still there", n)
	}
}

// A step that fails after moving a token has moved the action: the record is
// made from where the tokens got, not withheld for the error.
func TestActionStepFailingAfterAMoveRecordsIt(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		action race {
			attribute woken : Integer = 0;
			first start;
			fork split;
			action mark { assign woken := missingName + 1; }
			action idle;
			action after;
			join meet;
			done;
			succession first start then split;
			succession first split then mark;
			succession first split then idle;
			succession first idle then after;
			succession first after then meet;
			succession first mark then meet;
			succession first meet then done;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "race", ast.DefAction)
	exec, err := newActionExecutor(ctx, sym, nil)
	if err != nil {
		t.Fatalf("newActionExecutor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := exec.Step(); err != nil {
			t.Fatalf("Step %d: %v", i+1, err)
		}
	}
	if got := tokenNodes(exec); got != "idle mark" {
		t.Fatalf("tokens at %s after the fork, want idle mark", got)
	}

	// The record of the two steps that got here is set aside to see the failing step's own.
	exec.moved = false
	err = exec.Step()
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("Step = %v, want ErrUnresolvedReference from mark's body", err)
	}
	if got := tokenNodes(exec); got != "after mark" {
		t.Fatalf("tokens at %s after the failing step, want the idle token moved on to after", got)
	}
	if !exec.moved {
		t.Error("the step moved a token and failed, yet the action does not read as moved")
	}
}

// tokenNodes names the nodes the tokens sit at, sorted.
func tokenNodes(exec *ActionExecutor) string {
	names := make([]string, 0, len(exec.tokens))
	for _, token := range exec.tokens {
		names = append(names, ActionNodeName(token.Location))
	}
	slices.Sort(names)
	return strings.Join(names, " ")
}

// A change transition whose effect fails has left its source state: the machine
// has moved, and reads so.
func TestStateChangeTransitionFailingRecordsTheMove(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute counter : Integer = 0;
			entry; then idle;
			state idle;
			state busy;
			transition first idle accept when counter == 0 do assign counter := missingName + 1 then busy;
		}
	}`)
	if exec.moved || activeLeaf(exec) != "idle" {
		t.Fatalf("moved = %v at %s after initialize, want unmoved at idle", exec.moved, activeLeaf(exec))
	}
	fired, err := exec.PollChangeEvents()
	if !fired || !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("PollChangeEvents = %v, %v; want fired with ErrUnresolvedReference from the effect", fired, err)
	}
	if got := activeLeaf(exec); got == "idle" {
		t.Fatal("the machine is still at idle after the transition left it")
	}
	if !exec.moved {
		t.Error("the transition left idle and its effect failed, yet the machine does not read as moved")
	}
}
