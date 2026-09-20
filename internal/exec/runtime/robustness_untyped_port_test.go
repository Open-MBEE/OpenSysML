package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessUntypedPort exercises the failure modes of ports declared
// without a type, which materialize as Ports::Port objects so binding connectors join them.
func TestRuntimeRobustnessUntypedPort(t *testing.T) {
	t.Run("joined_to_no_receiving_port", testUntypedPortJoinedToNothing)
	t.Run("bound_to_no_feature", testUntypedPortBoundToNoFeature)
	t.Run("bound_inward_delivers", testUntypedPortBoundInwardDelivers)
}

// rigWithUntypedPorts is a sender wired to a box whose untyped boundary port is
// bound as given to the nested receiver; wiring names the rig's connector, if any.
func rigWithUntypedPorts(binding, wiring string) string {
	return `package test {
		item def Go;
		part def Sender {
			port tx;
			action def Fire {
				first start;
				then action go send new Go() via this.tx;
				then done;
			}
			perform action fire : Fire;
		}
		part def Receiver {
			port rx;
			exhibit state life {
				entry; then idle;
				state idle;
				state got;
				transition first idle accept Go via rx then got;
			}
		}
		part def Box {
			port cmd;
			part r : Receiver;
			` + binding + `
		}
		part def Rig {
			part s : Sender;
			part b : Box;
			` + wiring + `
		}
	}`
}

// testUntypedPortJoinedToNothing: a send via an untyped port no connector joins
// is refused as unroutable rather than dropped.
func testUntypedPortJoinedToNothing(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, rigWithUntypedPorts("bind cmd = r.rx;", ""), "test::Rig")
	if !errors.Is(err, ErrUnroutableSend) || !strings.Contains(err.Error(), `port "tx" is joined to no port that can receive it`) {
		t.Fatalf("error = %v, want ErrUnroutableSend over an unjoined untyped port", err)
	}
}

// testUntypedPortBoundToNoFeature: a binding whose end names no feature of the
// nested part fails as an unresolvable binding end when the send reaches it.
func testUntypedPortBoundToNoFeature(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, rigWithUntypedPorts("bind cmd = r.nowhere;", "connect s.tx to b.cmd;"), "test::Rig")
	if !errors.Is(err, ErrBindingEnd) || !strings.Contains(err.Error(), `"r.nowhere"`) {
		t.Fatalf("error = %v, want ErrBindingEnd naming r.nowhere", err)
	}
}

// testUntypedPortBoundInwardDelivers: the well-formed rig delivers the signal to the
// receiver through the bound untyped ports, so its machine leaves idle.
func testUntypedPortBoundInwardDelivers(t *testing.T) {
	ctx, rig, err := instantiateWithLibraries(t, rigWithUntypedPorts("bind cmd = r.rx;", "connect s.tx to b.cmd;"), "test::Rig")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	exec := objectMachine(t, instanceAtPath(t, ctx, rig, "b.r"), "")
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run receiver machine: %v", err)
	}
	if got := exec.FinalStateName(); got != "got" {
		t.Fatalf("receiver state = %q, want got", got)
	}
}
