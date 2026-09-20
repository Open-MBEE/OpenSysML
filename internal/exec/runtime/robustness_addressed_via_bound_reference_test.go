package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessAddressedViaBoundReference exercises the failure modes of
// a send addressed to a receiver over a via path rooted at a bound reference: the
// port and the receiver are the bound object's, and are refused under the path
// as written when that object lacks them.
func TestRuntimeRobustnessAddressedViaBoundReference(t *testing.T) {
	t.Run("receiver_the_bound_object_lacks", testAddressedViaBoundReferenceMissingReceiver)
	t.Run("port_the_bound_object_lacks", testAddressedViaBoundReferenceMissingPort)
	t.Run("receiver_of_the_sender_alone", testAddressedViaBoundReferenceSenderReceiver)
}

// hostAddressing is a host whose performed action sends the given addressed
// send with `dev` bound to its device, which listens on `rx` as `listener`.
func hostAddressing(send string) string {
	return `package test {
		item def Ping;
		port def PingOut { out item ping : Ping; }
		part def Device {
			port tx : PingOut;
			port rx : ~PingOut;
			connect tx to rx;
			exhibit state listener {
				entry; then idle;
				state idle;
				transition first idle accept Ping via rx then idle;
			}
		}
		part def Host {
			part device : Device;
			action def Fire {
				in ref dev : Device;
				first start;
				then action go ` + send + `
				then done;
			}
			action def Round {
				first start;
				then action fire : Fire { in ref dev = this.device; }
				then done;
			}
			perform action round : Round;
		}
	}`
}

// testAddressedViaBoundReferenceMissingReceiver: the receiver is looked up on
// the bound device, so a name it runs no behavior under is unreachable.
func testAddressedViaBoundReferenceMissingReceiver(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, hostAddressing(
		"send new Ping() via dev.tx to missing;",
	), "test::Host")
	var typed *UnreachableSendReceiverError
	if !errors.As(err, &typed) || typed.Port != "dev.tx" || typed.Receiver != "missing" {
		t.Fatalf("error = %v, want UnreachableSendReceiverError for missing through dev.tx", err)
	}
	if !errors.Is(err, ErrUnreachableSendReceiver) || !strings.Contains(err.Error(), `"missing" is not reachable through port "dev.tx"`) {
		t.Fatalf("error = %v, want the written path in the report", err)
	}
}

// testAddressedViaBoundReferenceMissingPort: the path names the bound device's
// port, so one it does not declare is refused under the path as written.
func testAddressedViaBoundReferenceMissingPort(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, hostAddressing(
		"send new Ping() via dev.nope to listener;",
	), "test::Host")
	var typed *UnknownSendPortError
	if !errors.As(err, &typed) || typed.Port != "dev.nope" || typed.Receiver != "listener" {
		t.Fatalf("error = %v, want UnknownSendPortError for dev.nope", err)
	}
	if !errors.Is(err, ErrSendViaUnknownPort) {
		t.Fatalf("error = %v, want ErrSendViaUnknownPort", err)
	}
}

// testAddressedViaBoundReferenceSenderReceiver: a receiver only the sender runs
// is no node of the bound device the port belongs to, so it is unreachable
// rather than delivered to the sender's own behavior.
func testAddressedViaBoundReferenceSenderReceiver(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, hostAddressing(
		"send new Ping() via dev.tx to round;",
	), "test::Host")
	var typed *UnreachableSendReceiverError
	if !errors.As(err, &typed) || typed.Port != "dev.tx" || typed.Receiver != "round" {
		t.Fatalf("error = %v, want UnreachableSendReceiverError for round through dev.tx", err)
	}
}
