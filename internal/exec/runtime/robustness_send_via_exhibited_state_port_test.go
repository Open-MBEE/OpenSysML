package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// TestRuntimeRobustnessSendViaExhibitedStatePort: `send … via p` through a port the state
// definition owns fails as a typed error naming the port when nothing routes it; never a panic.
func TestRuntimeRobustnessSendViaExhibitedStatePort(t *testing.T) {
	t.Run("unbound_state_port_is_unroutable", testUnboundStatePortIsUnroutable)
	t.Run("bound_state_port_without_a_connector_is_unroutable", testBoundStatePortWithoutAConnectorIsUnroutable)
	t.Run("state_port_that_fails_to_materialize_fails_the_send", testStatePortThatFailsToMaterializeFailsTheSend)
	t.Run("accept_via_this_names_the_performers_port_only", testAcceptViaThisNamesThePerformersPortOnly)
	t.Run("send_via_this_names_the_performers_port_only", testSendViaThisNamesThePerformersPortOnly)
}

const exhibitedStatePortHead = `
	package test {
		item def Ping;
		item def Pong;
		port def CommandPort { in item request : Ping; out item reply : Pong; }
		state def Responder {
			port command : CommandPort;
			port response : ~CommandPort;
			entry; then idle;
			state idle;
			state replied;
			transition first idle accept Ping via command
				do send new Pong() via response
				then replied;
		}
`

// testUnboundStatePortIsUnroutable: a state port bound and connected to nothing routes
// nowhere; the exhibiting part's same-named port is another feature.
func testUnboundStatePortIsUnroutable(t *testing.T) {
	src := exhibitedStatePortHead + `
		part def Tug {
			port command : CommandPort;
			port response : ~CommandPort;
			exhibit state m : Responder;
			interface wire connect response to command;
			perform action kick {
				first start;
				then action p send new Ping() via response;
				then done;
			}
		}
	}`
	_, _, err := instantiateInSource(t, src, "test::Tug")
	if !errors.Is(err, errUnroutableSend) {
		t.Fatalf("err = %v, want errUnroutableSend", err)
	}
	if !strings.Contains(err.Error(), `"response"`) {
		t.Errorf("err = %v, want it to name the port", err)
	}
}

// testBoundStatePortWithoutAConnectorIsUnroutable: a send through a state port bound to a
// part port nothing connects is unroutable as a typed error naming the port, not dropped.
func testBoundStatePortWithoutAConnectorIsUnroutable(t *testing.T) {
	src := exhibitedStatePortHead + `
		state def Greeter {
			port response : ~CommandPort;
			entry; then greeting;
			state greeting { entry send new Pong() via response; }
		}
		part def Tug {
			port response : ~CommandPort;
			exhibit state m : Greeter;
			bind m.response = response;
		}
	}`
	_, _, err := instantiateInSource(t, src, "test::Tug")
	if !errors.Is(err, errUnroutableSend) {
		t.Fatalf("err = %v, want errUnroutableSend", err)
	}
	if !strings.Contains(err.Error(), "response") {
		t.Errorf("err = %v, want it to name the port", err)
	}
}

// testStatePortThatFailsToMaterializeFailsTheSend: a state port whose value cannot
// be computed fails the send with that error, not a delivery through its binding.
func testStatePortThatFailsToMaterializeFailsTheSend(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		item def Ping;
		item def Pong;
		port def CommandPort { in item request : Ping; out item reply : Pong; }
		state def Responder {
			port command : CommandPort;
			port response : ~CommandPort = 1 / 0;
			entry; then idle;
			state idle;
			state replied;
			transition first idle accept Ping via command
				do send new Pong() via response
				then replied;
		}
		part def Tug {
			port command : CommandPort;
			port response : ~CommandPort;
			exhibit state m : Responder;
			bind m.response = response;
			interface wire connect response to command;
			perform action kick {
				first start;
				then action p send new Ping() via response;
				then done;
			}
		}
	}`
	_, _, err := instantiateInSource(t, src, "test::Tug")
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("err = %v, want the port's %v", err, ErrDivisionByZero)
	}
}

// exhibitedStatePortViaThis: the receive port and the reply port of two machines, connected
// directly on the performances; the vias are written from `this`, the performer, which has neither.
const exhibitedStatePortViaThis = `
	package test {
		private import ScalarValues::*;
		item def Ping;
		item def Pong;
		port def CommandPort { in item request : Ping; out item reply : Pong; }
		state def Counter {
			port ear : CommandPort;
			attribute heard : Integer = 0;
			entry; then waiting;
			state waiting;
			state counted;
			transition first waiting accept Pong via ear
				do assign heard := heard + 1
				then counted;
		}
`

// testAcceptViaThisNamesThePerformersPortOnly: `accept … via this.inbox` asks for the
// performer's port; a message at the performance's own `inbox` does not fire it.
func testAcceptViaThisNamesThePerformersPortOnly(t *testing.T) {
	src := exhibitedStatePortViaThis + `
		state def Responder {
			port inbox : CommandPort;
			port response : ~CommandPort;
			entry; then idle;
			state idle;
			state replied;
			transition first idle accept Ping via this.inbox
				do send new Pong() via response
				then replied;
		}
		part def Tug {
			port response : ~CommandPort;
			exhibit state m : Responder;
			exhibit state n : Counter;
			interface wire connect response to m.inbox;
			interface back connect m.response to n.ear;
			perform action kick {
				first start;
				then action p send new Ping() via response;
				then done;
			}
		}
	}`
	ctx, tug, err := instantiateInSource(t, src, "test::Tug")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	if got := heardBy(t, ctx, tug, "n"); got != 0 {
		t.Fatalf("n.heard = %d, want 0: the accept via this.inbox fired on the performance's port", got)
	}
}

// testSendViaThisNamesThePerformersPortOnly: `send … via this.response` asks for the
// performer's port; with none, the send is unroutable rather than re-rooted to the performance's.
func testSendViaThisNamesThePerformersPortOnly(t *testing.T) {
	src := exhibitedStatePortViaThis + `
		state def Greeter {
			port response : ~CommandPort;
			entry; then greeting;
			state greeting { entry send new Pong() via this.response; }
		}
		part def Tug {
			exhibit state m : Greeter;
			exhibit state n : Counter;
			interface back connect m.response to n.ear;
		}
	}`
	_, _, err := instantiateInSource(t, src, "test::Tug")
	if !errors.Is(err, errUnroutableSend) {
		t.Fatalf("err = %v, want errUnroutableSend", err)
	}
}

// heardBy reads the `heard` count of the machine the object exhibits under name.
func heardBy(t *testing.T, ctx *Context, obj *Instance, name string) int64 {
	t.Helper()
	fv, err := obj.GetFeatureValue(ctx, name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	held := fv.HeldValue()
	if held.Kind != ValInstance {
		t.Fatalf("%s holds %v, want an object", name, held)
	}
	machine, ok := ctx.Instance(held.Instance)
	if !ok {
		t.Fatalf("%s holds object #%d, not found", name, held.Instance)
	}
	heard, err := machine.GetFeatureValue(ctx, "heard")
	if err != nil {
		t.Fatalf("read %s.heard: %v", name, err)
	}
	v := soleElement(heard.HeldValue())
	if v.Kind != ValConst || v.Const.Kind != semantics.ValInt {
		t.Fatalf("%s.heard = %s, want an Integer", name, FormatValue(v))
	}
	return v.Const.Int
}
