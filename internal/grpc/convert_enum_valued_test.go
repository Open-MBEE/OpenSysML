package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// valuedEnumModel declares an enumeration whose literals are Integers, and an
// object holding one by name and one by the scalar it equals.
const valuedEnumModel = `
package D {
  private import ScalarValues::*;
  enum def Level :> Integer { low = 1; high = 3; }
  enum def Rank :> Integer { one = 1; three = 3; }
  part def Rover {
    attribute named : Level = Level::high;
    attribute cast : Level[0..1] = 3 as Level;
  }
}
`

// evaluateValued parses valuedEnumModel and evaluates expression in D.
func evaluateValued(t *testing.T, srv *Service, expression string) *pb.Value {
	t.Helper()
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: valuedEnumModel},
		ContentHash: "valued-enum",
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	resp, err := srv.Evaluate(context.Background(), &pb.EvaluateRequest{
		ModelHash:       parseResp.ModelHash,
		Expression:      expression,
		ContextSymbolId: "D",
	})
	if err != nil {
		t.Fatalf("Evaluate(%s) failed: %v", expression, err)
	}
	if resp.Error != "" {
		t.Fatalf("Evaluate(%s): %s", expression, resp.Error)
	}
	return resp.Result
}

// wantLevelHigh asserts pv is D::Level::high carrying the Integer 3 it equals.
func wantLevelHigh(t *testing.T, what string, pv *pb.Value) {
	t.Helper()
	lit := pv.GetEnumLiteral()
	if lit == nil {
		t.Fatalf("%s: got kind %T (%v), want enum_literal", what, pv.GetKind(), pv)
	}
	if lit.LiteralId != "D::Level::high" || lit.EnumerationId != "D::Level" || lit.Name != "Level::high" {
		t.Errorf("%s: got %+v, want D::Level::high of D::Level", what, lit)
	}
	if v := lit.GetValue(); v == nil || v.GetIntValue() != 3 {
		t.Errorf("%s: value: got %v, want int_value 3", what, v)
	}
}

// TestScalarValuedEnumLiteralCrossesAsLiteral verifies a literal of an
// enumeration that specializes Integer leaves the API as the literal it is, the
// scalar it equals alongside, whether named, held by a feature or cast to.
func TestScalarValuedEnumLiteralCrossesAsLiteral(t *testing.T) {
	srv := mustNewService(t, 10)
	for _, expr := range []string{"Level::high", "3 as Level", "Rover::named", "Rover::cast"} {
		wantLevelHigh(t, expr, evaluateValued(t, srv, expr))
	}
	// A bare Integer is not a literal, and Rank's equal literal is Rank's.
	if got := evaluateValued(t, srv, "3"); got.GetEnumLiteral() != nil || got.GetIntValue() != 3 {
		t.Errorf("3: got %v, want int_value 3", got)
	}
	if got := evaluateValued(t, srv, "Rank::three").GetEnumLiteral(); got == nil || got.LiteralId != "D::Rank::three" {
		t.Errorf("Rank::three: got %v, want D::Rank::three", got)
	}
	// A literal that is only its identity carries no value.
	idx, red := enumWireIndex(t, "D::Color::red")
	if lit := ValueToProto(runtime.NewEnumLiteral(red), idx).GetEnumLiteral(); lit.GetValue() != nil {
		t.Errorf("Color::red: value: got %v, want none", lit.GetValue())
	}
}

// TestScalarValuedEnumLiteralInstanceFeature verifies the literal reaches a
// client as a feature value of an instantiated object too.
func TestScalarValuedEnumLiteralInstanceFeature(t *testing.T) {
	resp := instantiate(t, valuedEnumModel, "valued-enum-instance", "D::Rover")
	for _, name := range []string{"named", "cast"} {
		fv := resp.Instance.FeatureValues[name]
		if fv == nil || fv.Error != "" {
			t.Fatalf("feature value %s: %v", name, fv)
		}
		wantLevelHigh(t, name, fv.Value)
	}
}

// TestScalarValuedEnumLiteralRoundTrip verifies the wire form comes back as the
// same literal computing as the same scalar, from the model when a runtime is at
// hand and from the wire's value when none is.
func TestScalarValuedEnumLiteralRoundTrip(t *testing.T) {
	srv := mustNewService(t, 10)
	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: valuedEnumModel},
		ContentHash: "valued-enum-roundtrip",
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	cached, ok := srv.cache.Get(parseResp.ModelHash)
	if !ok {
		t.Fatal("model not cached")
	}
	rt, idx := srv.newRuntime(cached), cached.Index
	sem := rt.Semantics()

	high := idx.LookupQualified("D::Level::high")
	if len(high) != 1 {
		t.Fatalf("D::Level::high: %d symbols", len(high))
	}
	original, _, err := rt.EnumerationLiteralValue(high[0])
	if err != nil {
		t.Fatalf("EnumerationLiteralValue: %v", err)
	}
	pv := ValueToProto(original, idx)
	wantLevelHigh(t, "Level::high", pv)

	for name, back := range map[string]func() (runtime.Value, error){
		"with runtime":    func() (runtime.Value, error) { return ProtoToRuntimeValue(rt, pv, idx, sem) },
		"without runtime": func() (runtime.Value, error) { return ProtoToValueIn(pv, idx, sem) },
	} {
		got, err := back()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got.EnumerationLiteral() != high[0] {
			t.Errorf("%s: literal: got %v, want D::Level::high", name, got.EnumerationLiteral())
		}
		if got.Kind != runtime.ValConst || got.Const.Int != 3 {
			t.Errorf("%s: scalar: got %v, want Integer 3", name, got)
		}
	}
}
