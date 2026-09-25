package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessBehaviorConnectionEnd exercises the failure modes of a
// behavior's own connection whose end starts at a parameter the behavior binds:
// the end is read through the binding, so what the binding holds decides them.
func TestRuntimeRobustnessBehaviorConnectionEnd(t *testing.T) {
	t.Run("end_rooted_at_a_value", testBehaviorConnectionEndRootedAtAValue)
	t.Run("end_rooted_at_nothing", testBehaviorConnectionEndRootedAtNothing)
}

// hostWiring is a host whose performed action connects `dev.p` to a listener
// through a parameter `snk` declared and bound as given, then sends via `dev.p`.
func hostWiring(param, binding string) string {
	return `package test {
		private import ScalarValues::*;
		item def Ping;
		port def PingOut { out item ping : Ping; }
		part def Device { port p : PingOut; }
		part def Listener {
			port local : ~PingOut;
			exhibit state listen {
				entry; then idle;
				state idle;
				transition first idle accept Ping via local then idle;
			}
		}
		part def Host {
			part device : Device;
			part listener : Listener;
			action def Fire {
				in ref dev : Device;
				` + param + `
				connect dev.p to snk.local;
				first start;
				then action go send new Ping() via dev.p;
				then done;
			}
			action def Round {
				first start;
				then action fire : Fire { in ref dev = this.device; ` + binding + ` }
				then done;
			}
			perform action round : Round;
		}
	}`
}

// testBehaviorConnectionEndRootedAtAValue: a parameter holding a number holds no
// object whose port the end could name; the run refuses rather than resolving the
// end as a feature of the sender.
func testBehaviorConnectionEndRootedAtAValue(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, hostWiring(
		"in snk : Integer;", "in snk = 4;",
	), "test::Host")
	if !errors.Is(err, ErrSendTargetNotObject) || !strings.Contains(err.Error(), `"snk" holds 4, which is no object to address "snk.local" to`) {
		t.Fatalf("error = %v, want ErrSendTargetNotObject over a number", err)
	}
}

// testBehaviorConnectionEndRootedAtNothing: an optional parameter bound to nothing
// holds no object whose port the end could name, so the send is refused as one
// addressing nothing instead of dropped or delivered to the sender.
func testBehaviorConnectionEndRootedAtNothing(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, hostWiring(
		"in ref snk : Listener[0..1];", "in ref snk = null;",
	), "test::Host")
	if !errors.Is(err, ErrSendTargetNotObject) || !strings.Contains(err.Error(), `"snk" holds null, which is no object to address "snk.local" to`) {
		t.Fatalf("error = %v, want ErrSendTargetNotObject over an empty parameter", err)
	}
}
