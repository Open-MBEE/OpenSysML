package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessAcceptPayload exercises an accept whose payload flows on
// through the node's output: a specialized signal satisfies a general accept
// and its attributes read through the pin; an accept nothing posts to is a
// typed deadlock; a send to an object running no behavior completes.
func TestRuntimeRobustnessAcceptPayload(t *testing.T) {
	t.Run("specialized_signal_flows_through_the_pin", testAcceptPayloadSpecializedSignalFlows)
	t.Run("unposted_accept_is_a_typed_deadlock", testAcceptPayloadUnpostedDeadlock)
	t.Run("send_to_an_idle_object_completes", testAcceptPayloadSendToIdleObject)
}

// signalPayloadModel runs one sender and one accept typed Reading in an action,
// the accepted value flowing to a consumer that doubles its level.
func signalPayloadModel(sender string) string {
	return `package test {
		private import ScalarValues::*;
		attribute def Reading { attribute level : Integer; }
		attribute def Calibrated :> Reading;
		action def Communicator {
			out total : Integer = 0;
			first start;
			action sender { ` + sender + ` }
			action receiver accept msg : Reading;
			action consumer { in value : Reading; assign total := value.level * 2; }
			done;
			flow receiver.msg to consumer.value;
			succession first start then sender;
			succession first sender then receiver;
			succession first receiver then consumer;
			succession first consumer then done;
		}
	}`
}

func testAcceptPayloadSpecializedSignalFlows(t *testing.T) {
	outputs, err := executeActionSource(t, "Communicator", signalPayloadModel("send new Calibrated(level = 8) to receiver;"))
	if err != nil {
		t.Fatal(err)
	}
	if got := intOutput(t, outputs, "total"); got != 16 {
		t.Fatalf("total = %d, want 16 from the Calibrated level accepted as a Reading", got)
	}
}

func testAcceptPayloadUnpostedDeadlock(t *testing.T) {
	_, err := executeActionSource(t, "Communicator", signalPayloadModel(""))
	if !errors.Is(err, ErrAcceptDeadlock) || !strings.Contains(err.Error(), "accept msg waiting since step 3 for a message of type Reading") {
		t.Fatalf("error = %v, want ErrAcceptDeadlock naming the waiting accept", err)
	}
}

func testAcceptPayloadSendToIdleObject(t *testing.T) {
	outputs, err := executeActionSource(t, "Notifier", `package test {
		private import ScalarValues::*;
		attribute def Go { attribute level : Integer; }
		part def Target;
		action def Notifier {
			out sent : Integer = 0;
			first start;
			action make { out target : Target = new Target(); }
			action notify { in target : Target; send new Go(level = 4) to target; assign sent := 4; }
			done;
			flow make.target to notify.target;
			succession first start then make;
			succession first make then notify;
			succession first notify then done;
		}
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := intOutput(t, outputs, "sent"); got != 4 {
		t.Fatalf("sent = %d, want 4", got)
	}
}
