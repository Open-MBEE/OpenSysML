package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

// executePerformedAction executes the action named by action, performed by an
// instance of the part named by owner: the shape of a behavior a part performs.
func executePerformedAction(t *testing.T, src, owner, action string) (map[string]Value, error) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	self, err := ctx.Instantiate(oneSymbol(t, idx, owner))
	if err != nil {
		t.Fatalf("instantiate %s: %v", owner, err)
	}
	return ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, action), self, nil)
}

const directedPorts = "\n\tport def Chan { out attribute v : Integer; }\n"

// A message traverses an end that can receive it: the conjugated `~Chan` end
// sees the definition's `out` feature as `in`, so a send through the plain end
// arrives there (SysML v2 §7.12.2, §7.15).
func TestSendReachesTheConjugatedEndOfAConnection(t *testing.T) {
	outputs, err := executeActionSource(t, "ship", `package P {`+directedPorts+`
		action ship {
			attribute got : Integer = 0;
			port src : Chan;
			port dst : ~Chan;
			connect src to dst;
			first start;
			action sender { send 11 via src; }
			action reader accept n : Integer via dst { assign got := n; }
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 11)
}

// The direction of a port's flow features decides what a send may traverse, so a
// send into an end that only carries outward is reported where it was written
// rather than delivered: `dst` is joined only to `src`, whose feature is `out`.
func TestSendIntoAnOutboundOnlyEndIsATypedError(t *testing.T) {
	_, err := executeActionSource(t, "ship", `package P {`+directedPorts+`
		action ship {
			port src : Chan;
			port dst : ~Chan;
			connect src to dst;
			first start;
			action sender { send 11 via dst; }
			done;
			succession first start then sender;
			succession first sender then done;
		}
	}`)
	if !errors.Is(err, ErrUnroutableSend) {
		t.Fatalf("expected ErrUnroutableSend, got: %v", err)
	}
	if !strings.Contains(err.Error(), "src") {
		t.Errorf("expected the refusing end in the message, got: %v", err)
	}
}

// A port a connector joins nothing to reaches no one, which the send reports
// rather than dropping the message silently.
func TestSendThroughAnUnjoinedPortIsATypedError(t *testing.T) {
	_, err := executeActionSource(t, "ship", `package P {
		action ship {
			port lonely;
			first start;
			action sender { send 42 via lonely; }
			done;
			succession first start then sender;
			succession first sender then done;
		}
	}`)
	if !errors.Is(err, ErrUnroutableSend) {
		t.Fatalf("expected ErrUnroutableSend, got: %v", err)
	}
}

// A nested port is joined as the path it was written with, so a message routed
// through `p.q` reaches the end joined to `p.q` and not one joined to another
// port named `q` (SysML v2 §7.12).
func TestSendReachesANestedPortByItsPath(t *testing.T) {
	outputs, err := executeActionSource(t, "ship", `package P {
		action ship {
			attribute got : Integer = 0;
			port p { port q; }
			port other { port q; }
			port sink;
			connect p.q to sink;
			first start;
			action sender { send 7 via p.q; }
			action reader accept n : Integer via sink { assign got := n; }
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 7)
}

// A behavior reaches the ports of the part performing it, which is the normal
// shape of a model: the connector and both its ends are declared on the part,
// and the action only sends through them (SysML v2 §7.16).
func TestSendReachesThePortsOfThePerformingPart(t *testing.T) {
	outputs, err := executePerformedAction(t, `package P {
		part def Node {
			port src;
			port dst;
			connect src to dst;
		}
		part node : Node {
			action ship {
				attribute got : Integer = 0;
				first start;
				action sender { send 3 via src; }
				action reader accept n : Integer via dst { assign got := n; }
				done;
				succession first start then sender;
				succession first sender then reader;
				succession first reader then done;
			}
		}
	}`, "P::node", "P::node::ship")
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 3)
}

// A routed send from a behavior performed by a materialized part keeps that
// part's identity while constraining delivery to the named receiver.
func TestRoutedSendKeepsPerformingObjectIdentity(t *testing.T) {
	outputs, err := executePerformedAction(t, `package P {
		part def Node {
			port src;
			port dst;
			connect src to dst;
		}
		part node : Node {
			action ship {
				attribute got : Integer = 0;
				first start;
				action sender { send 3 via src to reader; }
				action reader accept n : Integer via dst { assign got := n; }
				done;
				succession first start then sender;
				succession first sender then reader;
				succession first reader then done;
			}
		}
	}`, "P::node", "P::node::ship")
	if err != nil {
		t.Fatalf("execute routed action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 3)
}

// The part a behavior is declared in performs it by owning it, so its ports are
// reachable even when no instance was created to perform the behavior.
func TestSendReachesEnclosingPartPortsWithoutAnInstance(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		part node {
			port src;
			port dst;
			connect src to dst;
			action ship {
				attribute got : Integer = 0;
				first start;
				action sender { send 4 via src; }
				action reader accept n : Integer via dst { assign got := n; }
				done;
				succession first start then sender;
				succession first sender then reader;
				succession first reader then done;
			}
		}
	}`))
	outputs, err := ctx.ExecuteAction(oneSymbol(t, idx, "P::node::ship"))
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 4)
}

// A state, a region and a transition are not what performs a behavior, so the
// search for the performing part passes through them: a send written in a state
// machine reaches the enclosing part's ports as one written in an action does.
func TestSendFromAStateMachineReachesEnclosingPartPorts(t *testing.T) {
	bodies := map[string]string{
		"state entry": `
			state waiting { entry { send Ping() via src; } }
			succession first start then waiting;
			transition first waiting accept Ping via dst do assign got := 1 then done;`,
		"transition effect": `
			state waiting;
			state sent;
			succession first start then waiting;
			transition go first waiting do send Ping() via src then sent;
			transition first sent accept Ping via dst do assign got := 1 then done;`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
				item def Ping;
				part node {
					port src;
					port dst;
					connect src to dst;
					state radio {
						attribute got : Integer = 0;
						entry; then start;
						state start;
						`+body+`
					}
				}
			}`))
			outputs, err := ctx.ExecuteState(oneSymbol(t, idx, "P::node::radio"))
			if err != nil {
				t.Fatalf("execute state machine: %v", err)
			}
			assertIntOutput(t, outputs, "got", 1)
		})
	}
}

// A connector may join a port of the performing part to a port of the behavior
// itself, and routing must see both ends: the part's end is reached through the
// performer, the behavior's through its own body.
func TestSendCrossesFromAPartPortToABehaviorPort(t *testing.T) {
	outputs, err := executePerformedAction(t, `package P {
		part def Node { port src; }
		part node : Node {
			action ship {
				attribute got : Integer = 0;
				port local;
				connect src to local;
				first start;
				action sender { send 5 via src; }
				action reader accept n : Integer via local { assign got := n; }
				done;
				succession first start then sender;
				succession first sender then reader;
				succession first reader then done;
			}
		}
	}`, "P::node", "P::node::ship")
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 5)
}

// An interface usage joins its ends the way a connection does, so a message
// routes over an interface-typed connection between two conjugate ports
// (SysML v2 §7.12.3).
func TestSendRoutesOverAnInterfaceTypedConnection(t *testing.T) {
	outputs, err := executeActionSource(t, "ship", `package P {`+directedPorts+`
		interface def Link {
			end port from : Chan;
			end port to : ~Chan;
		}
		action ship {
			attribute got : Integer = 0;
			port src : Chan;
			port dst : ~Chan;
			interface link : Link connect src to dst;
			first start;
			action sender { send 13 via src; }
			action reader accept n : Integer via dst { assign got := n; }
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	if err != nil {
		t.Fatalf("execute action: %v", err)
	}
	assertIntOutput(t, outputs, "got", 13)
}

// nestedObject reads the object a chain of features of inst holds.
func nestedObject(t *testing.T, ctx *Context, inst *Instance, path ...string) *Instance {
	t.Helper()
	for _, name := range path {
		held, ok, err := ctx.fvObject(inst, name)
		if err != nil || !ok {
			t.Fatalf("read %s: held = %v, err = %v", name, ok, err)
		}
		inst = held
	}
	return inst
}

// ownerConnectedParts is a site whose connector joins the port of a part nested
// two levels down to the port of a sibling part, with the receiving port's type
// left to the caller so the same model states a receiving and an outbound end.
func ownerConnectedParts(receiving string) string {
	return `package P {` + directedPorts + `
		part def Console { port command : Chan; }
		part def Bay { part console : Console; }
		part def Unit { port command : ` + receiving + `; }
		part def Site {
			part bay : Bay;
			part unit : Unit;
			connect bay.console.command to unit.command;
		}
		part site : Site {
			action ship {
				first start;
				action sender { send 9 via command; }
				done;
				succession first start then sender;
				succession first sender then done;
			}
		}
	}`
}

// A connector its owner declares joins two objects, so a send through the port
// of one arrives at the port of the other, held to that object's identity rather
// than to the sender's (SysML v2 §7.16). The connector names the sending port by
// the path from itself, so a part nested deeper is reached through that path.
func TestSendCrossesAnOwnerConnectionToASiblingPart(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, ownerConnectedParts("~Chan")))
	site, err := ctx.Instantiate(oneSymbol(t, idx, "P::site"))
	if err != nil {
		t.Fatalf("instantiate site: %v", err)
	}
	console := nestedObject(t, ctx, site, "bay", "console")
	unit := nestedObject(t, ctx, site, "unit")
	if _, err := ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "P::site::ship"), console, nil); err != nil {
		t.Fatalf("execute action: %v", err)
	}
	pending := ctx.PendingMessages()
	if len(pending) != 1 {
		t.Fatalf("pending messages = %v, want one delivered across the connection", pending)
	}
	if got := pending[0]; got.Object != unit.ID || got.Port != "command" {
		t.Errorf("delivered to object %d port %q, want object %d port %q",
			got.Object, got.Port, unit.ID, "command")
	}
}

// A part joining its own port to the port of a part it holds delegates inward:
// the send routes over the part's own connection, and the copy is held to the
// nested part's identity rather than to the sender's (SysML v2 §7.16).
func TestSendDelegatesToTheNestedPartItIsConnectedTo(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {`+directedPorts+`
		part def Unit { port command : ~Chan; }
		part def Bay {
			port command : Chan;
			part unit : Unit;
			connect command to unit.command;
		}
		part bay : Bay {
			action ship {
				first start;
				action sender { send 9 via command; }
				done;
				succession first start then sender;
				succession first sender then done;
			}
		}
	}`))
	bay, err := ctx.Instantiate(oneSymbol(t, idx, "P::bay"))
	if err != nil {
		t.Fatalf("instantiate bay: %v", err)
	}
	unit := nestedObject(t, ctx, bay, "unit")
	if _, err := ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "P::bay::ship"), bay, nil); err != nil {
		t.Fatalf("execute action: %v", err)
	}
	pending := ctx.PendingMessages()
	if len(pending) != 1 {
		t.Fatalf("pending messages = %v, want one delivered inward", pending)
	}
	if got := pending[0]; got.Object != unit.ID || got.Port != "command" {
		t.Errorf("delivered to object %d port %q, want object %d port %q",
			got.Object, got.Port, unit.ID, "command")
	}
}

// The direction of the peer end's flow features decides an owner's connection as
// it does a behavior's own: a sibling port that only carries outward receives
// nothing, so the send is reported rather than delivered.
func TestSendCrossingToAnOutboundOnlySiblingEndIsATypedError(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, ownerConnectedParts("Chan")))
	site, err := ctx.Instantiate(oneSymbol(t, idx, "P::site"))
	if err != nil {
		t.Fatalf("instantiate site: %v", err)
	}
	console := nestedObject(t, ctx, site, "bay", "console")
	_, err = ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "P::site::ship"), console, nil)
	if !errors.Is(err, ErrUnroutableSend) {
		t.Fatalf("execute action: err = %v, want %v", err, ErrUnroutableSend)
	}
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("pending messages = %v, want none", ctx.PendingMessages())
	}
}

// A port a behavior declares under the name of the performer's connected port
// shadows it: the send resolves the behavior's own port, so the owner's
// connector receives nothing, and the unjoined local port is reported.
func TestBehaviorLocalPortShadowsThePerformersConnectedPort(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {`+directedPorts+`
		item def Ping;
		part def Console { port command : Chan; }
		part def Unit { port command : ~Chan; }
		part def Site {
			part console : Console;
			part unit : Unit;
			connect console.command to unit.command;
		}
		part site : Site;
		state def Radio {
			port command : Chan;
			entry; then start;
			state start;
			state sent;
			transition go first start do send Ping() via command then sent;
		}
	}`))
	site, err := ctx.Instantiate(oneSymbol(t, idx, "P::site"))
	if err != nil {
		t.Fatalf("instantiate site: %v", err)
	}
	console := nestedObject(t, ctx, site, "console")
	exec, err := newStateExecutor(ctx, oneSymbol(t, idx, "P::Radio"), console)
	if err != nil {
		t.Fatalf("create state executor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrUnroutableSend) {
		t.Fatalf("run state machine: err = %v, want %v", err, ErrUnroutableSend)
	}
	if pending := ctx.PendingMessages(); len(pending) != 0 {
		t.Errorf("pending messages = %v, want none over the shadowed connector", pending)
	}
}

// A port a behavior inherits from its generalization shadows the performer's
// connected port the same way one it declares does: the send resolves the
// inherited port, so the owner's connector receives nothing.
func TestInheritedBehaviorPortShadowsThePerformersConnectedPort(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {`+directedPorts+`
		item def Ping;
		part def Console { port command : Chan; }
		part def Unit { port command : ~Chan; }
		part def Site {
			part console : Console;
			part unit : Unit;
			connect console.command to unit.command;
		}
		part site : Site;
		state def BaseRadio {
			port command : Chan;
		}
		state def Radio :> BaseRadio {
			entry; then start;
			state start;
			state sent;
			transition go first start do send Ping() via command then sent;
		}
	}`))
	site, err := ctx.Instantiate(oneSymbol(t, idx, "P::site"))
	if err != nil {
		t.Fatalf("instantiate site: %v", err)
	}
	console := nestedObject(t, ctx, site, "console")
	exec, err := newStateExecutor(ctx, oneSymbol(t, idx, "P::Radio"), console)
	if err != nil {
		t.Fatalf("create state executor: %v", err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrUnroutableSend) {
		t.Fatalf("run state machine: err = %v, want %v", err, ErrUnroutableSend)
	}
	if pending := ctx.PendingMessages(); len(pending) != 0 {
		t.Errorf("pending messages = %v, want none over the shadowed connector", pending)
	}
}

// A port declaring no flow features constrains no direction, so it receives
// whatever reaches it in either direction (see also TestSendViaPortRoutesInEitherDirection).
func TestUndirectedPortsReceiveInEitherDirection(t *testing.T) {
	_, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P { }`))
	conns := []lower.Connection{{Ends: []string{"a", "b"}}}
	for _, from := range []string{"a", "b"} {
		receiving, outbound := ctx.receivingEnds(conns, from, nil)
		if len(receiving) != 1 || len(outbound) != 0 {
			t.Errorf("from %s: receiving = %v, outbound = %v, want one receiving end", from, receiving, outbound)
		}
	}
}
