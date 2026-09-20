package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessTransitionVia exercises the failure modes of a transition
// whose `via` path starts at a reference parameter of the machine that holds no
// object, or an object with no port the rest of the path names.
func TestRuntimeRobustnessTransitionVia(t *testing.T) {
	t.Run("bound_root_holds_nothing", testTransitionViaRootHoldsNothing)
	t.Run("bound_chain_names_no_port", testTransitionViaChainNamesNoPort)
}

// rigListeningVia is a listener whose machine takes a signal via the given path
// from a reference parameter bound as given, fed by a sender through its own port.
func rigListeningVia(param, binding, via string) string {
	return `package test {
		item def Go;
		item def Ack;
		port def AckOut { out item ack : Ack; }
		port def AckIn { in item ack : Ack; }
		part def Peer { port hears : AckIn; }
		part def Listener {
			port own : AckIn;
			ref part partner : Peer;
			state def Life {
				` + param + `
				entry; then waiting;
				state waiting;
				transition first waiting accept Ack via ` + via + ` then heard;
				state heard;
			}
			exhibit state life : Life { ` + binding + ` }
		}
		part def Sender {
			port tx : AckOut;
			exhibit state life {
				entry; then idle;
				state idle;
				transition first idle accept Go then sent;
				state sent { entry send new Ack() via tx; }
			}
		}
		part def Rig {
			part listener : Listener { :>> partner = live; }
			part live : Peer;
			part sender : Sender;
			connect sender.tx to listener.own;
		}
	}`
}

// signalListener instantiates the rig and has the sender fire: delivering the
// acknowledgement resolves the listener's via path, whose failure the delivery reports.
func signalListener(t *testing.T, model string) error {
	t.Helper()
	ctx, rig, err := instantiateWithLibraries(t, model, "test::Rig")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	listener := objectMachine(t, instanceAtPath(t, ctx, rig, "listener"), "")
	if err := listener.RunToCompletion(); err != nil {
		t.Fatalf("run listener machine: %v", err)
	}
	sender := objectMachine(t, instanceAtPath(t, ctx, rig, "sender"), "")
	if err := sender.RunToCompletion(); err != nil {
		t.Fatalf("run sender machine: %v", err)
	}
	sender.SendSignal("Go", nil)
	return sender.RunToCompletion()
}

// testTransitionViaRootHoldsNothing: a parameter bound to nothing names no object
// whose port the path could reach, so the machine refuses rather than waiting forever.
func testTransitionViaRootHoldsNothing(t *testing.T) {
	err := signalListener(t, rigListeningVia(
		"in ref context : Peer[0..1];", "in :>> context = null;", "context.hears",
	))
	if !errors.Is(err, ErrSendTargetNotObject) || !strings.Contains(err.Error(), `"context" holds null, which is no object`) {
		t.Fatalf("error = %v, want ErrSendTargetNotObject over an empty parameter", err)
	}
}

// testTransitionViaChainNamesNoPort: a path from the bound object through a
// feature it has none of is refused, naming the missing feature.
func testTransitionViaChainNamesNoPort(t *testing.T) {
	err := signalListener(t, rigListeningVia(
		"in ref context : Peer;", "in :>> context = partner;", "context.nowhere.hears",
	))
	if !errors.Is(err, ErrNoSuchFeature) || !strings.Contains(err.Error(), `feature "nowhere" not found`) {
		t.Fatalf("error = %v, want ErrNoSuchFeature over a path naming no feature", err)
	}
}
