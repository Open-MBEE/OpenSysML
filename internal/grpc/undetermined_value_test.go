package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

const undeterminedModel = `package Demo {
	private import ScalarValues::*;
	private import SequenceFunctions::*;
	part def D;
	attribute u;
	part rack {
		part slots[3] : D;
		part gear[1..*] : D;
		part loose[0..2] : D;
	}
}
`

func evaluateUndetermined(t *testing.T, srv *Service, hash, expression string) *pb.EvaluateResponse {
	t.Helper()
	resp, err := srv.Evaluate(context.Background(), &pb.EvaluateRequest{ModelHash: hash, Expression: expression})
	if err != nil {
		t.Fatalf("Evaluate(%s): %v", expression, err)
	}
	return resp
}

// A model-level expression over a feature the model gives no value is answered, not
// refused: the result crosses as the undetermined arm with its reason and count.
func TestEvaluate_UnboundFeatureIsSentUndetermined(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, undeterminedModel, "verify-undetermined")

	for _, expression := range []string{"Demo::u", "Demo::u + 5", "(Demo::u > 3) and true"} {
		resp := evaluateUndetermined(t, srv, hash, expression)
		if resp.Error != "" {
			t.Errorf("%s: reported %q, want an undetermined result", expression, resp.Error)
			continue
		}
		u := resp.GetResult().GetUndetermined()
		if u == nil {
			kind, value := describeValue(resp.GetResult())
			t.Errorf("%s: %s %v, want undetermined", expression, kind, value)
			continue
		}
		if !strings.Contains(u.GetReason(), "u has no value") {
			t.Errorf("%s: reason = %q, want it to name the unbound feature", expression, u.GetReason())
		}
		if u.GetCount().GetLower() != "1" || u.GetCount().GetUpper() != "1" {
			t.Errorf("%s: count = %v, want 1..1", expression, u.GetCount())
		}
	}
}

// An open multiplicity crosses with the bounds the model states, so a client can
// tell an unfixed count from an unbound scalar.
func TestEvaluate_OpenCardinalityIsSentUndeterminedWithBounds(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, undeterminedModel, "verify-undetermined-bounds")

	resp := evaluateUndetermined(t, srv, hash, "Demo::rack.gear")
	u := resp.GetResult().GetUndetermined()
	if u == nil {
		t.Fatalf("rack.gear: %v (%q), want undetermined", resp.GetResult(), resp.Error)
	}
	if u.GetCount().GetLower() != "1" || u.GetCount().GetUpper() != "*" {
		t.Errorf("rack.gear: count = %v, want 1..*", u.GetCount())
	}

	fixed := evaluateUndetermined(t, srv, hash, "SequenceFunctions::size(Demo::rack.slots)")
	if fixed.Error != "" || fixed.GetResult().GetIntValue() != 3 {
		t.Errorf("size(rack.slots) = %v (%q), want 3", fixed.GetResult(), fixed.Error)
	}
}

// A constant operand that fixes a Boolean form answers even beside an unknown.
func TestEvaluate_ConstantOperandFoldsBesideUndetermined(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, undeterminedModel, "verify-undetermined-fold")

	for expression, want := range map[string]bool{
		"(Demo::u > 3) and false":     false,
		"(Demo::u == 1) or true":      true,
		"(Demo::u == 1) implies true": true,
	} {
		resp := evaluateUndetermined(t, srv, hash, expression)
		got, ok := resp.GetResult().GetKind().(*pb.Value_BoolValue)
		if resp.Error != "" || !ok || got.BoolValue != want {
			t.Errorf("%s = %v (%q), want %v", expression, resp.GetResult(), resp.Error, want)
		}
	}
}

// The undetermined arm is never the unset one: a materialized feature holding no
// value and a model-level answer the model leaves open are told apart on the wire.
func TestValueToProto_UndeterminedIsNotUnset(t *testing.T) {
	one := semantics.Bound{Value: 1, Known: true}
	val := runtime.NewUndeterminedValue("u has no value in the model", semantics.Range{Lower: one, Upper: one})
	got := ValueToProtoIn(nil, val, nil)
	u, ok := got.GetKind().(*pb.Value_Undetermined)
	if !ok {
		t.Fatalf("kind = %T, want undetermined", got.GetKind())
	}
	if u.Undetermined.GetReason() != "u has no value in the model" {
		t.Errorf("reason = %q", u.Undetermined.GetReason())
	}
	if c := u.Undetermined.GetCount(); c.GetLower() != "1" || c.GetUpper() != "1" {
		t.Errorf("count = %v, want 1..1", c)
	}
}

// Undetermined is something to read and not to supply, as unset is.
func TestProtoToValue_RejectsUndetermined(t *testing.T) {
	arm := &pb.Value{Kind: &pb.Value_Undetermined{Undetermined: &pb.Undetermined{Reason: "x"}}}
	if _, err := ProtoToValueIn(arm, nil, nil); !errors.Is(err, ErrUndeterminedNotAccepted) {
		t.Errorf("err = %v, want %v", err, ErrUndeterminedNotAccepted)
	}
	seq := &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{
		Elements: []*pb.Value{{Kind: &pb.Value_IntValue{IntValue: 1}}, arm},
	}}}
	if _, err := ProtoToValueIn(seq, nil, nil); !errors.Is(err, ErrUndeterminedNotAccepted) {
		t.Errorf("in a sequence: err = %v, want %v", err, ErrUndeterminedNotAccepted)
	}
}

// The arm is a wire-visible addition, so it is advertised as a capability and
// withheld as an unsupported null from a client that did not negotiate it.
func TestCapabilities_IncludeUndeterminedValue(t *testing.T) {
	found := false
	for _, name := range Capabilities() {
		found = found || name == CapabilityUndeterminedValue
	}
	if !found {
		t.Errorf("capabilities = %v, want one named %q", Capabilities(), CapabilityUndeterminedValue)
	}

	srv := mustNewServiceWithout(t, CapabilityUndeterminedValue)
	value := &pb.Value{Kind: &pb.Value_Undetermined{Undetermined: &pb.Undetermined{Reason: "u has no value in the model"}}}
	srv.filterValueCapabilities(value)
	if !strings.Contains(value.GetNull(), runtime.UndeterminedText) || !strings.Contains(value.GetNull(), "u has no value") {
		t.Errorf("value = %v, want an unsupported null naming the reason", value)
	}
}
