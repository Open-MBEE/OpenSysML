package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessViaBoundPort exercises the failure modes of a via path that is
// a single name the behavior binds: the binding must hold exactly one port object.
func TestRuntimeRobustnessViaBoundPort(t *testing.T) {
	t.Run("bound_reference_holds_a_part", testViaBoundReferenceHoldsAPart)
	t.Run("bound_reference_holds_a_value", testViaBoundReferenceHoldsAValue)
	t.Run("bound_reference_holds_two_ports", testViaBoundReferenceHoldsTwoPorts)
}

// hostViaBinding is a host whose performed action sends `via` the given parameter,
// which the caller binds as given.
func hostViaBinding(parameter, binding string) string {
	return `package test {
		private import ScalarValues::*;
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
			port tx : PingOut;
			part device : Device;
			part spare : Device;
			attribute count : Integer = 3;
			action def Fire {
				` + parameter + `
				first start;
				then action go send new Ping() via tx;
				then done;
			}
			action def Round {
				first start;
				then action fire : Fire { ` + binding + ` }
				then done;
			}
			perform action round : Round;
		}
	}`
}

// testViaBoundReferenceHoldsAPart: a bound name holding an object that is no port is
// refused rather than falling back to the performer's same-named port.
func testViaBoundReferenceHoldsAPart(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, hostViaBinding(
		"in ref tx : Device;", "in ref :>> tx = device;",
	), "test::Host")
	var typed *ViaNotPortError
	if !errors.As(err, &typed) || typed.Via != "tx" {
		t.Fatalf("error = %v, want ViaNotPortError for tx", err)
	}
	if !errors.Is(err, ErrSendViaNotPort) || !strings.Contains(err.Error(), `"tx" holds`) {
		t.Fatalf("error = %v, want ErrSendViaNotPort naming the binding", err)
	}
}

// testViaBoundReferenceHoldsAValue: a bound name holding a value rather than an
// object cannot be sent via.
func testViaBoundReferenceHoldsAValue(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, hostViaBinding(
		"in tx : Integer;", "in :>> tx = count;",
	), "test::Host")
	var typed *SendTargetValueError
	if !errors.As(err, &typed) || typed.Target != "tx" || typed.Name != "tx" || typed.Value != "3" {
		t.Fatalf("error = %v, want SendTargetValueError for tx holding 3", err)
	}
	if !errors.Is(err, ErrSendTargetNotObject) {
		t.Fatalf("error = %v, want ErrSendTargetNotObject", err)
	}
}

// testViaBoundReferenceHoldsTwoPorts: a bound name holding more than one port names no
// single port to send via.
func testViaBoundReferenceHoldsTwoPorts(t *testing.T) {
	_, _, err := instantiateWithLibraries(t, hostViaBinding(
		"in ref port tx : PingOut[2];", "in ref port :>> tx = (device.tx, spare.tx);",
	), "test::Host")
	var typed *SendTargetValueError
	if !errors.As(err, &typed) || typed.Target != "tx" || typed.Name != "tx" {
		t.Fatalf("error = %v, want SendTargetValueError for tx holding two ports", err)
	}
	if !errors.Is(err, ErrSendTargetNotObject) {
		t.Fatalf("error = %v, want ErrSendTargetNotObject", err)
	}
}
