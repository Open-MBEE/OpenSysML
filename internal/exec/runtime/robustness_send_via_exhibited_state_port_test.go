package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessSendViaExhibitedStatePort: `send … via p` through a port the state
// definition owns fails as a typed error naming the port when nothing routes it; never a panic.
func TestRuntimeRobustnessSendViaExhibitedStatePort(t *testing.T) {
	t.Run("unbound_state_port_is_unroutable", testUnboundStatePortIsUnroutable)
	t.Run("bound_state_port_without_a_connector_is_unroutable", testBoundStatePortWithoutAConnectorIsUnroutable)
	t.Run("state_port_that_fails_to_materialize_fails_the_send", testStatePortThatFailsToMaterializeFailsTheSend)
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
