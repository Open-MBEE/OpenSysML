package runtime

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestRuntimeRobustnessSendToObjectIdentity exercises a send whose receiver is an
// expression rather than a name: evaluating it to no object, to a value that is
// none, or to a destroyed one is a typed error, and a routed send whose receiver
// object is reached by no connection is refused — never a panic or a hang.
func TestRuntimeRobustnessSendToObjectIdentity(t *testing.T) {
	t.Run("target_expression_yielding_no_object", testSendToNoObject)
	t.Run("target_expression_yielding_a_data_value", testSendToDataValue)
	t.Run("target_expression_yielding_a_destroyed_object", testSendToDestroyedObject)
	t.Run("via_receiver_object_no_connection_reaches", testSendViaUnreachedReceiver)
}

// sendToObjectModel is a fleet whose build action creates two cars and then runs
// ping, the statement under test.
func sendToObjectModel(ping string) string {
	return `package test {
		private import ScalarValues::*;
		private import OccurrenceFunctions::*;
		private import SequenceFunctions::*;
		private import ControlFunctions::*;
		item def Ping;
		part def Car {
			exhibit state listening {
				entry; then waiting;
				state waiting;
				accept Ping then heard;
				state heard;
			}
		}
		part def Fleet {
			part cars : Car[0..*];
			ref part spare : Car[0..1];
			attribute sameObject : Boolean = false;
			perform action build {
				first start;
				then action make {
					assign cars := (cars, new Car());
					assign cars := (cars, new Car());
					assign spare := new Car();
				}
				then action ping {
					` + ping + `
				}
				then done;
			}
		}
	}`
}

// instantiateBounded runs Instantiate on its own goroutine so a hang fails the
// case instead of stalling the suite, and a panic in it fails the case.
func instantiateBounded(t *testing.T, src, fqn string) error {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := oneSymbol(t, idx, fqn)
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("Instantiate panicked: %v", r)
			}
		}()
		_, err := ctx.Instantiate(sym)
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(30 * time.Second):
		t.Fatalf("Instantiate(%s) did not return within 30s", fqn)
		return nil
	}
}

// A selection admitting nothing yields no object, which is no target.
func testSendToNoObject(t *testing.T) {
	err := instantiateBounded(t, sendToObjectModel(`send new Ping() to cars->select { in c; false };`), "test::Fleet")
	if !errors.Is(err, ErrSendTargetNotObject) {
		t.Fatalf("error = %v, want %v", err, ErrSendTargetNotObject)
	}
}

// An expression yielding a data value names no object to address.
func testSendToDataValue(t *testing.T) {
	err := instantiateBounded(t, sendToObjectModel(`send new Ping() to 3;`), "test::Fleet")
	if !errors.Is(err, ErrSendTargetNotObject) {
		t.Fatalf("error = %v, want %v", err, ErrSendTargetNotObject)
	}
}

// An object destroyed before the send addresses it is refused as destroyed.
func testSendToDestroyedObject(t *testing.T) {
	err := instantiateBounded(t, sendToObjectModel(
		`assign sameObject := destroy(cars#(2)) === cars#(2);
		send new Ping() to cars#(2);`), "test::Fleet")
	if !errors.Is(err, ErrOccurrenceDestroyed) {
		t.Fatalf("error = %v, want %v", err, ErrOccurrenceDestroyed)
	}
}

// A routed send whose receiver object holds no connected port reaches nothing.
func testSendViaUnreachedReceiver(t *testing.T) {
	model := `package test {
		item def Ping;
		port def PingPort { in item ping : Ping; }
		part def Car {
			port p : PingPort;
			exhibit state listening {
				entry; then waiting;
				state waiting;
				accept Ping via p then heard;
				state heard;
			}
		}
		part def Fleet {
			part cars : Car[0..*];
			ref part spare : Car[0..1];
			port out : ~PingPort;
			connect out to cars.p;
			perform action build {
				first start;
				then action make {
					assign cars := (cars, new Car());
					assign spare := new Car();
				}
				then action ping {
					send new Ping() via out to spare;
				}
				then done;
			}
		}
	}`
	err := instantiateBounded(t, model, "test::Fleet")
	if !errors.Is(err, ErrUnreachableSendReceiver) {
		t.Fatalf("error = %v, want %v", err, ErrUnreachableSendReceiver)
	}
}
