package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

// A path through a package-level action or state usage reaches what its one performance
// holds, as a path through a part does; a bare `a` stays a receiver addressed by name.
func TestAddressedSendReachesAPortUnderAPackageLevelPerformance(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		item def Ping;
		port def Pt;
		action def A { part device { port inPort : Pt; } }
		action a : A;
		action listen { first start; done; succession first start then done; }
	}`))
	for _, target := range []string{"a.device.inPort", "test::a.device.inPort"} {
		send := lower.Send{Target: target, TargetPath: true, Scope: DeclScope(oneSymbol(t, idx, "test::listen"))}
		if err := ctx.post(nil, nil, Message{SignalType: "Ping"}, send, nil, nil, nil); err != nil {
			t.Fatalf("post to %s: %v", target, err)
		}
	}
	val, err := evalIn(t, ctx, lookupOne(t, idx, "test").Scope, "a.device")
	if err != nil || val.Kind != ValInstance {
		t.Fatalf("a.device = %v, %v, want the performance's device", val, err)
	}
	msgs := ctx.PendingMessages()
	if len(msgs) != 2 {
		t.Fatalf("pending = %d messages, want 2", len(msgs))
	}
	for _, msg := range msgs {
		if !msg.reaches("", "inPort", val.Instance) {
			t.Errorf("%+v does not reach inPort of a.device (object %d)", msg, val.Instance)
		}
	}
}

// A path through an optional package-level performance nothing binds (`action a : A[0..1];`)
// is unroutable: the usage denotes no object, and a send makes none up.
func TestAddressedSendThroughAnOptionalPerformanceMakesNoObject(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		item def Ping;
		port def Pt;
		action def A { port inPort : Pt; }
		action a : A[0..1];
		action listen { first start; done; succession first start then done; }
	}`))
	send := lower.Send{Target: "a.inPort", TargetPath: true, Scope: DeclScope(oneSymbol(t, idx, "test::listen"))}
	err := ctx.post(nil, nil, Message{SignalType: "Ping"}, send, nil, nil, nil)
	if !errors.Is(err, errUnroutableSend) {
		t.Fatalf("post to a.inPort: %v, want errUnroutableSend", err)
	}
	if n := countInstancesOfType(ctx, "A"); n != 0 {
		t.Errorf("%d A made up for an optional usage by a send, want none", n)
	}
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("an unroutable send posted %+v", ctx.PendingMessages())
	}
}
