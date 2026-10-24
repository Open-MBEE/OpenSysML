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
	t.Run("accept_payload_without_a_value", testAcceptPayloadWithoutAValue)
	t.Run("accept_payload_read_before_it_is_bound", testAcceptPayloadReadBeforeItIsBound)
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
	if got := soleIntOutput(t, outputs, "total"); got != 16 {
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
	if got := soleIntOutput(t, outputs, "sent"); got != 4 {
		t.Fatalf("sent = %d, want 4", got)
	}
}

// testAcceptPayloadWithoutAValue: a message carrying no value and naming no
// signal definition gives the accept's payload name nothing to bind, which is
// reported rather than bound to nothing.
func testAcceptPayloadWithoutAValue(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		item def Ping;
		action pipeline {
			first start;
			action reader accept p : Ping;
			done;
			succession first start then reader;
			succession first reader then done;
		}
	}`))
	exec, err := ctx.CreateActionExecutor(oneSymbol(t, idx, "P::pipeline"))
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	ctx.PostMessage(Message{SignalType: "Ping", Target: "reader", Payload: map[string]Value{}})
	err = exec.RunToCompletion()
	if err == nil {
		t.Fatal("expected the payload-less message to be reported")
	}
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("expected ErrNoValue, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Ping") {
		t.Errorf("error does not name the accepted signal: %v", err)
	}
}

// testAcceptPayloadReadBeforeItIsBound: the payload is a declaration of the body
// wherever the body resolves, so a node running before the accept binds it
// resolves the name — and a payload's bare parameter admits no value, so the read
// yields the empty sequence its assign target refuses, not an unresolved name.
func testAcceptPayloadReadBeforeItIsBound(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			attribute seen : Integer = 0;
			first start;
			action reader { assign seen := msg; }
			action waiter accept msg : Integer;
			done;
			succession first start then reader;
			succession first reader then waiter;
			succession first waiter then done;
		}
	}`)
	// A payload names no multiplicity of its own, so §7.6.3 gives it [0..*]:
	// unbound it resolves to the empty sequence, which a [1..1] assign target
	// refuses by count — reported, and not read as a name that fails to resolve.
	if err == nil {
		t.Fatal("expected the unbound payload read to fail")
	}
	if errors.Is(err, ErrUnresolvedReference) {
		t.Errorf("a declared payload was reported as unresolved: %v", err)
	}
	if !strings.Contains(err.Error(), "seen") || !strings.Contains(err.Error(), "multiplicity") {
		t.Errorf("error = %v, want the assign target's count refusal naming seen", err)
	}
}
