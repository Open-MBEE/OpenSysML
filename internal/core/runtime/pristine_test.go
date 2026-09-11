package runtime

import (
	"errors"
	"strings"
	"testing"

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
