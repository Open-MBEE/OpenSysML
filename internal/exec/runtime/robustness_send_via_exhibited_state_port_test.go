package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessSendViaExhibitedStatePort exercises `send … via p` where
// `p` is a port the state definition owns and a part exhibits the machine: a
// port nothing binds or connects is unroutable as a typed error naming it, and
// so is one bound to a part port that no connector joins. Never a panic or a hang.
func TestRuntimeRobustnessSendViaExhibitedStatePort(t *testing.T) {
	t.Run("unbound_state_port_is_unroutable", testUnboundStatePortIsUnroutable)
	t.Run("bound_state_port_without_a_connector_is_unroutable", testBoundStatePortWithoutAConnectorIsUnroutable)
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

// testUnboundStatePortIsUnroutable: the state definition's own port, bound to
// nothing and connected to nothing, routes nowhere; the exhibiting part's
// same-named port is another feature, so its interface does not carry the send.
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

// testBoundStatePortWithoutAConnectorIsUnroutable: binding the state's port to
// the part's makes the send leave the part's port; with that port connected to
// nothing, it is unroutable as a typed error naming the port, not dropped.
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
