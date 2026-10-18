package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// TestRuntimeRobustnessSendRouting covers send and accept routing: addressees, ports that
// fail to materialize, fan-out and messages that cannot be delivered.
func TestRuntimeRobustnessSendRouting(t *testing.T) {
	t.Run("send_reaches_only_its_addressee", testSendReachesOnlyItsAddressee)
	t.Run("accept_of_unsent_type", testAcceptOfUnsentTypeReports)
	t.Run("send_via_unconnected_port", testSendViaUnconnectedPort)
	t.Run("send_via_connector_into_an_empty_part", testSendViaConnectorIntoAnEmptyPart)
	t.Run("send_via_bound_boundary_port_joined_to_nothing", testSendViaBoundBoundaryPortJoinedToNothing)
	t.Run("send_fan_out_to_a_port_that_fails_to_materialize", testSendFanOutToAPortThatFailsToMaterialize)
	t.Run("accept_via_a_port_that_fails_to_materialize", testAcceptViaAPortThatFailsToMaterialize)
	t.Run("action_accept_via_a_port_that_fails_to_materialize", testActionAcceptViaAPortThatFailsToMaterialize)
	t.Run("send_addressed_to_an_unreachable_target", testSendAddressedToAnUnreachableTarget)
	t.Run("routed_send_via_unknown_port", testRoutedSendViaUnknownPort)
	t.Run("routed_send_port_type_mismatch", testRoutedSendPortTypeMismatch)
	t.Run("routed_send_port_type_match", testRoutedSendPortTypeMatch)
	t.Run("routed_send_scalar_typed_flow_mismatch", testRoutedSendScalarTypedFlowMismatch)
	t.Run("routed_send_unreachable_receiver", testRoutedSendUnreachableReceiver)
	t.Run("routed_send_receiver_name_mismatch_deadlock", testRoutedSendReceiverNameMismatchDeadlock)
	t.Run("send_addressed_through_several_occurrences", testSendAddressedThroughSeveralOccurrences)
	t.Run("send_addressed_to_an_object_that_cannot_be_built", testSendAddressedToAnObjectThatCannotBeBuilt)
	t.Run("send_addressed_to_a_part_no_sibling_takes", testSendAddressedToAPartNoSiblingTakes)
	t.Run("injected_message_names_a_receiver_no_accept_has", testInjectedMessageNamesAReceiverNoAcceptHas)
}

// testSendViaUnconnectedPort: a port with no connection reaches no one, so the
// send itself is undeliverable — which must be reported where it was written
// rather than left for the accept waiting on it to time out as a deadlock.
func testSendViaUnconnectedPort(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			port outPort;
			port inPort;
			first start;
			action sender { send 42 via outPort; }
			action reader accept msg : Integer via inPort;
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error: nothing connects outPort to inPort")
	}
	if !errors.Is(err, errUnroutableSend) {
		t.Errorf("expected errUnroutableSend, got: %v", err)
	}
}

// testSendViaConnectorIntoAnEmptyPart: the connector joins the sender's port to
// the port of a part it holds, but the part holds no object this run, so nothing
// is behind the end and the send is reported rather than delivered to a port path
// no consumer reads.
func testSendViaConnectorIntoAnEmptyPart(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {`+directedPorts+`
		part def Unit { port command : ~Chan; }
		part def Bay {
			port command : Chan;
			part unit : Unit[0];
			connect command to unit.command;
		}
		part bay : Bay {
			action ship {
				first start;
				action sender { send 9 via command; }
				done;
				succession first start then sender;
				succession first sender then done;
			}
		}
	}`))
	bay, err := ctx.Instantiate(oneSymbol(t, idx, "P::bay"))
	if err != nil {
		t.Fatalf("instantiate bay: %v", err)
	}
	_, err = ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "P::bay::ship"), bay, nil)
	if !errors.Is(err, errUnroutableSend) {
		t.Fatalf("execute action: err = %v, want %v", err, errUnroutableSend)
	}
	if pending := ctx.PendingMessages(); len(pending) != 0 {
		t.Errorf("pending messages = %+v, want none", pending)
	}
}

// testSendViaBoundBoundaryPortJoinedToNothing: the inner part's port is bound to
// the assembly's boundary port, but nothing in the context joins that boundary
// port, so the send reaches no receiving port. It is reported where the inner
// machine sent it, the same as an unconnected port of the sender's own, rather
// than dropped silently — and, as the inner part runs with the context it is
// created under, the context's creation is what reports it.
func testSendViaBoundBoundaryPortJoinedToNothing(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {`+directedPorts+`
		part def Inner {
			port out : Chan;
			exhibit state sm {
				entry; then sending;
				state sending { entry send 9 via out; }
			}
		}
		part def Assembly {
			port boundary : Chan;
			part child : Inner;
			bind boundary = child.out;
		}
		part def Env { port in : ~Chan; }
		part ctx {
			part asm : Assembly;
			part env : Env;
		}
	}`))
	_, err := ctx.Instantiate(oneSymbol(t, idx, "P::ctx"))
	if !errors.Is(err, errUnroutableSend) {
		t.Fatalf("instantiate ctx: err = %v, want %v", err, errUnroutableSend)
	}
	if pending := ctx.PendingMessages(); len(pending) != 0 {
		t.Errorf("pending messages = %+v, want none", pending)
	}
	if got := len(ctx.instances); got != 0 {
		t.Errorf("%d object(s) survive the failed creation, want none", got)
	}
}

// testSendFanOutToAPortThatFailsToMaterialize: a send fanning out over two
// connectors, where the second receiving port cannot be materialized, is
// reported as that failure and leaves no copy queued for the first — a retry
// must not find the earlier copy already there.
func testSendFanOutToAPortThatFailsToMaterialize(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;`+directedPorts+`
		part def Good { port in : ~Chan; }
		part def Bad { port in : ~Chan = 1 / 0; }
		part def Hub {
			port command : Chan;
			part good : Good;
			part bad : Bad;
			connect command to good.in;
			connect command to bad.in;
		}
		part hub : Hub {
			action ship {
				first start;
				action sender { send 9 via command; }
				done;
				succession first start then sender;
				succession first sender then done;
			}
		}
	}`))
	hub, err := ctx.Instantiate(oneSymbol(t, idx, "P::hub"))
	if err != nil {
		t.Fatalf("instantiate hub: %v", err)
	}
	_, err = ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "P::hub::ship"), hub, nil)
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("execute action: err = %v, want the port's %v", err, ErrDivisionByZero)
	}
	if pending := ctx.PendingMessages(); len(pending) != 0 {
		t.Errorf("pending messages = %+v, want none", pending)
	}
}

// testAcceptViaAPortThatFailsToMaterialize: a machine whose accept names a port
// that cannot be materialized leaves a message of another signal in flight
// untouched, and reports the port's failure when a message of its own signal
// arrives, rather than consuming either silently.
func testAcceptViaAPortThatFailsToMaterialize(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;`+directedPorts+`
		part def Listener {
			port in : ~Chan = 1 / 0;
			exhibit state sm {
				entry; then Idle;
				state Idle;
				accept v : Integer via in then Got;
				state Got;
			}
		}
		part listener : Listener;
	}`))
	listener, err := ctx.Instantiate(oneSymbol(t, idx, "P::listener"))
	if err != nil {
		t.Fatalf("instantiate listener: %v", err)
	}
	exec, err := ctx.CreateStateExecutorFor(oneSymbol(t, idx, "P::Listener::sm"), listener)
	if err != nil {
		t.Fatalf("create state executor: %v", err)
	}
	// Delivered to a port object by identity, under a name the accept does not
	// use, so only materializing `in` can tell whether it is the same port.
	viaPort := func(signal string, value Value) Message {
		return Message{SignalType: signal, Port: "other", Object: listener.ID, PortID: -1,
			Delivery: deliverPort, Value: &value}
	}
	ctx.PostMessage(viaPort("String", NewStringValue("not for you")))
	if exec.HasPendingSignal() {
		t.Fatal("a String is not the Integer the accept names, yet the machine claims it")
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run with only a String in flight: %v", err)
	}
	if pending := ctx.PendingMessages(); len(pending) != 1 || pending[0].SignalType != "String" {
		t.Fatalf("pending messages = %+v, want the String still in flight", pending)
	}
	ctx.PostMessage(viaPort("Integer", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 4}}))
	if !exec.HasPendingSignal() {
		t.Fatal("an Integer that may be for the accept is not claimed")
	}
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("run with an Integer in flight: err = %v, want the port's %v", err, ErrDivisionByZero)
	}
	if visits := exec.getStateVisits(); len(visits) != 1 || visits[0] != "Idle" {
		t.Errorf("state visits = %v, want [Idle]", visits)
	}
	if pending := ctx.PendingMessages(); len(pending) != 2 || pending[0].SignalType != "String" || pending[1].SignalType != "Integer" {
		t.Errorf("pending messages = %+v, want the String then the Integer still in flight", pending)
	}
}

// testActionAcceptViaAPortThatFailsToMaterialize: the action-node counterpart
// of the machine case above: a parked accept whose port cannot be materialized
// leaves a message of another signal in flight and stays parked, and reports
// the port's failure only for a message of its own signal.
func testActionAcceptViaAPortThatFailsToMaterialize(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;`+directedPorts+`
		part def Listener {
			port in : ~Chan = 1 / 0;
			action listen {
				first start;
				action reader accept v : Integer via in;
				done;
				succession first start then reader;
				succession first reader then done;
			}
		}
		part listener : Listener;
	}`))
	listener, err := ctx.Instantiate(oneSymbol(t, idx, "P::listener"))
	if err != nil {
		t.Fatalf("instantiate listener: %v", err)
	}
	exec, err := ctx.CreateActionExecutorFor(oneSymbol(t, idx, "P::Listener::listen"), listener)
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	for i := 0; i < 10 && exec.State() != StateWaiting; i++ {
		if err := exec.Step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	if exec.State() != StateWaiting {
		t.Fatalf("state = %v, want %v", exec.State(), StateWaiting)
	}
	viaPort := func(signal string, value Value) Message {
		return Message{SignalType: signal, Target: "reader", Port: "other", Object: listener.ID, PortID: -1,
			Delivery: deliverPort, Value: &value}
	}
	ctx.PostMessage(viaPort("String", NewStringValue("not for you")))
	if exec.HasPendingSignal() {
		t.Fatal("a String is not the Integer the accept names, yet the action claims it")
	}
	if err := exec.Step(); err != nil {
		t.Fatalf("step with only a String in flight: %v", err)
	}
	if exec.State() != StateWaiting {
		t.Fatalf("state = %v, want still %v", exec.State(), StateWaiting)
	}
	if pending := ctx.PendingMessages(); len(pending) != 1 || pending[0].SignalType != "String" {
		t.Fatalf("pending messages = %+v, want the String still in flight", pending)
	}
	ctx.PostMessage(viaPort("Integer", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 4}}))
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("run with an Integer in flight: err = %v, want the port's %v", err, ErrDivisionByZero)
	}
	if pending := ctx.PendingMessages(); len(pending) != 2 || pending[0].SignalType != "String" || pending[1].SignalType != "Integer" {
		t.Errorf("pending messages = %+v, want the String then the Integer still in flight", pending)
	}
}

func testRoutedSendViaUnknownPort(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			first start;
			action sender { send 42 via missing to reader; }
			action reader accept msg : Integer;
			done;
			succession first start then sender;
			succession first sender then done;
		}
	}`)
	var typed *unknownSendPortError
	if !errors.As(err, &typed) {
		t.Fatalf("expected unknownSendPortError, got: %v", err)
	}
	if !errors.Is(err, errSendViaUnknownPort) {
		t.Errorf("expected errSendViaUnknownPort, got: %v", err)
	}
	if errors.Is(err, errUnroutableSend) {
		t.Errorf("unknown routed port must not be errUnroutableSend: %v", err)
	}
	if !strings.Contains(err.Error(), `"missing"`) || !strings.Contains(err.Error(), `"reader"`) {
		t.Errorf("error = %v, want port and receiver names", err)
	}
}

func testRoutedSendPortTypeMismatch(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		item def IntMessage;
		item def TextMessage;
		port def IntegerPort { in item text : TextMessage; }
		action pipeline {
			port outPort;
			port inPort : IntegerPort;
			connect outPort to inPort;
			first start;
			action sender { send IntMessage via outPort to reader; }
			action reader accept msg : Integer via inPort;
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	var typed *sendPortTypeMismatchError
	if !errors.As(err, &typed) {
		t.Fatalf("expected sendPortTypeMismatchError, got: %v", err)
	}
	if !errors.Is(err, errSendPortTypeMismatch) {
		t.Errorf("expected errSendPortTypeMismatch, got: %v", err)
	}
	if !strings.Contains(err.Error(), `"outPort"`) ||
		!strings.Contains(err.Error(), `"reader"`) {
		t.Errorf("error = %v, want port and receiver names", err)
	}
}

func testRoutedSendPortTypeMatch(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		item def IntMessage;
		port def IntegerPort { in item value : IntMessage; }
		action pipeline {
			port outPort;
			port inPort : IntegerPort;
			connect outPort to inPort;
			first start;
			action sender { send IntMessage via outPort to reader; }
			action reader accept : IntMessage via inPort;
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	if err != nil {
		t.Fatalf("execute routed send through matching typed flow: %v", err)
	}
}

func testRoutedSendScalarTypedFlowMismatch(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		item def Integer;
		item def TextMessage;
		port def TextPort { in item text : TextMessage; }
		action pipeline {
			port outPort;
			port inPort : TextPort;
			connect outPort to inPort;
			first start;
			action sender { send 42 via outPort to reader; }
			action reader accept msg : Integer via inPort;
			done;
			succession first start then sender;
			succession first sender then reader;
			succession first reader then done;
		}
	}`)
	var typed *sendPortTypeMismatchError
	if !errors.As(err, &typed) {
		t.Fatalf("expected scalar sendPortTypeMismatchError, got: %v", err)
	}
	if !errors.Is(err, errSendPortTypeMismatch) {
		t.Errorf("expected errSendPortTypeMismatch for scalar message, got: %v", err)
	}
}

func testRoutedSendUnreachableReceiver(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			port outPort;
			port inPort;
			connect outPort to inPort;
			first start;
			action sender { send 42 via outPort to missing; }
			done;
			succession first start then sender;
			succession first sender then done;
		}
	}`)
	var typed *unreachableSendReceiverError
	if !errors.As(err, &typed) {
		t.Fatalf("expected unreachableSendReceiverError, got: %v", err)
	}
	if !errors.Is(err, errUnreachableSendReceiver) {
		t.Errorf("expected errUnreachableSendReceiver, got: %v", err)
	}
	if !strings.Contains(err.Error(), `"outPort"`) ||
		!strings.Contains(err.Error(), `"missing"`) {
		t.Errorf("error = %v, want port and receiver names", err)
	}
}

func testRoutedSendReceiverNameMismatchDeadlock(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		action pipeline {
			port outPort;
			port inPort;
			connect outPort to inPort;
			first start;
			action sender { send 42 via outPort to receiver; }
			action receiver;
			action sibling accept msg : Integer via inPort;
			done;
			succession first start then sender;
			succession first sender then sibling;
			succession first sibling then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected a deadlock: sibling must not consume receiver's message")
	}
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("expected ErrAcceptDeadlock, got: %v", err)
	}
}

// testSendAddressedToAnUnreachableTarget: a target reaching no port of an object
// the sender can address is reported where it was written rather than delivered
// to whatever else carries the last segment's name.
func testSendAddressedToAnUnreachableTarget(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		port def PingPort { in item ping : Integer; }
		part def Leaf { attribute count : Integer = 0; }
		part def Node {
			port inPort : PingPort;
			part leaf : Leaf;
		}
		part alpha : Node;
		action pipeline {
			first start;
			action sender { send 42 to alpha.leaf.count; }
			done;
			succession first start then sender;
			succession first sender then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error: alpha.leaf.count is no port of any object")
	}
	if !errors.Is(err, errUnroutableSend) {
		t.Errorf("expected errUnroutableSend, got: %v", err)
	}
}

// testSendAddressedThroughSeveralOccurrences: a path led by a part naming three
// occurrences reaches no one object, which must be reported rather than
// attributed to the sending object and delivered to nobody.
func testSendAddressedThroughSeveralOccurrences(t *testing.T) {
	_, err := executeActionSource(t, "pipeline", `package P {
		port def PingPort { in item ping : Integer; }
		part def Leaf { port inPort : PingPort; }
		part nodes : Leaf[3];
		action pipeline {
			first start;
			action sender { send 42 to nodes.inPort; }
			done;
			succession first start then sender;
			succession first sender then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error: nodes names three occurrences, not one addressee")
	}
	if !errors.Is(err, errUnroutableSend) {
		t.Errorf("expected errUnroutableSend, got: %v", err)
	}
}

// testSendAddressedToAPartNoSiblingTakes: a message addressed to a part belongs
// to that object, so a sibling accept of the sending behavior cannot take it and
// the run reports the accept it is left waiting on.
func testSendAddressedToAPartNoSiblingTakes(t *testing.T) {
	_, err := executeActionSource(t, "main", `package P {
		item def Ping;
		part def R;
		part receiver : R;
		action main {
			first start;
			action s { send Ping() to receiver; }
			action other accept p : Ping;
			done;
			succession first start then s;
			succession first s then other;
			succession first other then done;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error: `other` is no addressee of the message sent to receiver")
	}
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Errorf("expected ErrAcceptDeadlock, got: %v", err)
	}
}

// testInjectedMessageNamesAReceiverNoAcceptHas: a message injected from outside
// the model is held to the receiver it names, so an accept of another name waits
// on rather than consumes it, and the run reports that wait.
func testInjectedMessageNamesAReceiverNoAcceptHas(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package P {
		private import ScalarValues::*;
		action pipeline {
			first start;
			action other accept n : Integer;
			done;
			succession first start then other;
			succession first other then done;
		}
	}`))
	exec, err := ctx.CreateActionExecutor(oneSymbol(t, idx, "P::pipeline"))
	if err != nil {
		t.Fatalf("create action executor: %v", err)
	}
	one := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	ctx.PostMessage(Message{SignalType: "Integer", Target: "reader", Value: &one})
	err = exec.RunToCompletion()
	if err == nil {
		t.Fatal("expected an error: `other` is not the receiver the message names")
	}
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Errorf("expected ErrAcceptDeadlock, got: %v", err)
	}
}

// testSendAddressedToAnObjectThatCannotBeBuilt: locating an addressee can fail
// on its own terms — an exhausted budget, or a feature value the walk cannot read — and
// each must be reported as that rather than as an address naming no port.
func testSendAddressedToAnObjectThatCannotBeBuilt(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			port def PingPort { in item ping : Integer; }
			part def Node {
				port inPort : PingPort;
				attribute a = b + 1.0;
				attribute b = a + 1.0;
				action listen { first start; done; succession first start then done; }
			}
			part alpha : Node;
		}
	`))
	scope := DeclScope(oneSymbol(t, idx, "test::Node::listen"))

	ctx.maxSteps = 0
	send := lower.Send{Target: "alpha.inPort", TargetPath: true, Scope: scope}
	err := ctx.post(nil, nil, Message{SignalType: "Integer"}, send, nil, nil, nil)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("budget exhausted while building alpha: %v, want ErrStepLimitExceeded", err)
	}
	if errors.Is(err, errUnroutableSend) {
		t.Errorf("an exhausted budget was reported as a bad address: %v", err)
	}

	ctx.maxSteps = DefaultMaxSteps
	alpha := instanceOfUsage(t, ctx, idx, "test::alpha")
	send = lower.Send{Target: "a.inPort", TargetPath: true, Scope: scope}
	err = ctx.post(nil, nil, Message{SignalType: "Integer"}, send, nil, alpha, nil)
	if !errors.Is(err, ErrCyclicFeatureValue) {
		t.Errorf("walking through a cyclic derived feature value: %v, want ErrCyclicFeatureValue", err)
	}
	if errors.Is(err, errUnroutableSend) {
		t.Errorf("an unreadable feature value was reported as a bad address: %v", err)
	}
	if len(ctx.PendingMessages()) != 0 {
		t.Errorf("a send that never found its addressee posted %+v", ctx.PendingMessages())
	}
}
