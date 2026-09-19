package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessSendTarget exercises the failure modes of a send
// addressing a parameter of the sending action that holds no object to
// deliver to, or an object with no feature the rest of the target names.
func TestRuntimeRobustnessSendTarget(t *testing.T) {
	t.Run("bound_target_holds_a_value", testSendTargetHoldsAValue)
	t.Run("bound_target_holds_nothing", testSendTargetHoldsNothing)
	t.Run("bound_target_chain_names_no_feature", testSendTargetChainNamesNoFeature)
}

// hostNotifying is a host whose performed action notifies through a parameter
// bound as given, sending to the given target.
func hostNotifying(param, binding, target string) string {
	return `package test {
		private import ScalarValues::*;
		item def Go;
		part def Worker {
			exhibit state listen {
				entry; then idle;
				state idle;
				transition first idle accept Go then idle;
			}
		}
		part def Host {
			part worker : Worker;
			action def Notify {
				` + param + `
				first start;
				then action poke send new Go() to ` + target + `;
				then done;
			}
			action def Round {
				first start;
				then action notify : Notify { ` + binding + ` }
				then done;
			}
			perform action round : Round;
		}
	}`
}

// testSendTargetHoldsAValue: a parameter holding a number addresses no object;
// the run refuses rather than treating the name as a feature of the sender.
func testSendTargetHoldsAValue(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, hostNotifying(
		"in recipient : Integer;", "in recipient = 4;", "recipient",
	), "test::Host")
	if !errors.Is(err, ErrSendTargetNotObject) || !strings.Contains(err.Error(), `"recipient" holds 4, which is no object to address "recipient" to`) {
		t.Fatalf("error = %v, want ErrSendTargetNotObject over a number", err)
	}
}

// testSendTargetHoldsNothing: an optional parameter bound to nothing addresses
// no object, so the send is refused instead of dropped.
func testSendTargetHoldsNothing(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, hostNotifying(
		"in recipient : Worker[0..1];", "in recipient = null;", "recipient",
	), "test::Host")
	if !errors.Is(err, ErrSendTargetNotObject) || !strings.Contains(err.Error(), `"recipient" holds null, which is no object`) {
		t.Fatalf("error = %v, want ErrSendTargetNotObject over an empty parameter", err)
	}
}

// testSendTargetChainNamesNoFeature: a chain from a bound object reaching no
// port or object is unroutable, named as the target was written.
func testSendTargetChainNamesNoFeature(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, hostNotifying(
		"in recipient : Worker;", "in recipient = this.worker;", "recipient.nowhere",
	), "test::Host")
	if !errors.Is(err, ErrUnroutableSend) || !strings.Contains(err.Error(), `"recipient.nowhere" names no port of an object`) {
		t.Fatalf("error = %v, want ErrUnroutableSend over a chain naming no feature", err)
	}
}
