package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// quantityValueTypeWireModel holds a quantity as energy and takes one back as
// torque, energy or any quantity: the same dimension, sibling value types.
const quantityValueTypeWireModel = `
package M {
  private import SI::*;
  private import Quantities::*;

  attribute def Work :> ISQ::EnergyValue;
  attribute e : ISQ::EnergyValue = 5.0 [J];
  attribute w : Work = 2.0 [J];
  attribute es : ISQ::EnergyValue[*] = (1.0 [J], 2.0 [J]);
  attribute computed = e + 0.0 [J];
  attribute bare = 5.0 [J];

  calc def AsTorque { in x : ISQ::TorqueValue[1]; return : ScalarQuantityValue = x; }
  calc asTorque : AsTorque;
  calc def AsEnergy { in x : ISQ::EnergyValue[1]; return : ScalarQuantityValue = x; }
  calc asEnergy : AsEnergy;
  calc def AsAny { in x : ScalarQuantityValue[1]; return : ScalarQuantityValue = x; }
  calc asAny : AsAny;

  part rig { part motor { attribute energy : ISQ::EnergyValue = 5.0 [J]; attribute spare = 5.0 [J]; attribute derived : ISQ::EnergyValue = energy + spare; } }
}
`

// A quantity crosses with the quantity value type it is held as and reads back as held,
// so the sibling type refuses it sent back; one held as no type crosses with none.
func TestQuantityValueTypeRoundTrip(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustParse(t, srv, quantityValueTypeWireModel)
	cached, _ := srv.cache.Get(modelHash)
	idx, sem := cached.Index, NewSymbolContext(cached.Index).Semantics
	ctx := context.Background()

	sent := func(expr string) *pb.Value {
		t.Helper()
		pv := mustEvaluate(t, srv, modelHash, expr)
		if pv.GetQuantity() == nil {
			t.Fatalf("%s crossed as %T: %v", expr, pv.GetKind(), pv)
		}
		return pv
	}
	for _, tc := range []struct{ expr, valueType string }{
		{"M::e", "ISQThermodynamics::EnergyValue"},
		{"M::w", "M::Work"},
		{"M::computed", ""},
		{"M::bare", ""},
		{"5.0 [SI::J]", ""},
	} {
		pv := sent(tc.expr)
		if got := pv.GetQuantity().GetValueType(); got != tc.valueType {
			t.Errorf("%s crossed with value_type %q, want %q", tc.expr, got, tc.valueType)
		}
		back, err := protoconv.ProtoToValueIn(pv, idx, sem)
		if err != nil {
			t.Fatalf("ProtoToValueIn(%s): %v", tc.expr, err)
		}
		held := ""
		if kind := back.QuantityKind(); kind != nil {
			held = symbols.FQNOf(kind)
		}
		if held != tc.valueType {
			t.Errorf("%s read back held as %q, want %q", tc.expr, held, tc.valueType)
		}
	}

	elements := mustEvaluate(t, srv, modelHash, "M::es").GetSequence().GetElements()
	if len(elements) != 2 {
		t.Fatalf("M::es crossed as %d elements, want 2", len(elements))
	}
	for i, el := range elements {
		if got := el.GetQuantity().GetValueType(); got != "ISQThermodynamics::EnergyValue" {
			t.Errorf("M::es#(%d) crossed with value_type %q, want ISQThermodynamics::EnergyValue", i+1, got)
		}
	}

	invoke := func(calc string, arg *pb.Value) *pb.EvaluateCalcResponse {
		t.Helper()
		resp, err := srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "M::" + calc, Arguments: []*pb.Value{arg}})
		if err != nil {
			t.Fatalf("EvaluateCalc(%s): %v", calc, err)
		}
		return resp
	}
	for _, tc := range []struct {
		expr, calc string
		refused    bool
	}{
		{"M::e", "asTorque", true},
		{"M::w", "asTorque", true},
		{"M::e", "asEnergy", false},
		{"M::w", "asEnergy", false},
		{"M::e", "asAny", false},
		{"M::computed", "asTorque", false},
		{"M::bare", "asTorque", false},
	} {
		resp := invoke(tc.calc, sent(tc.expr))
		if strings.Contains(resp.GetError(), "type mismatch") != tc.refused {
			t.Errorf("%s(%s sent back): error = %q, want refused = %v", tc.calc, tc.expr, resp.GetError(), tc.refused)
		}
		if !tc.refused && soleElement(t, tc.calc, resp.Result).GetQuantity() == nil {
			t.Errorf("%s(%s sent back) = %v, want the quantity", tc.calc, tc.expr, resp.Result)
		}
	}
}

// A quantity naming a value type it cannot be held as is refused with a typed
// error, never read as held as none or as another type.
func TestQuantityValueTypeIsChecked(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustParse(t, srv, quantityValueTypeWireModel)
	cached, _ := srv.cache.Get(modelHash)
	idx, sem := cached.Index, NewSymbolContext(cached.Index).Semantics

	joules := func(valueType string) *pb.Value {
		pv := mustEvaluate(t, srv, modelHash, "M::bare")
		pv.GetQuantity().ValueType = valueType
		return pv
	}
	for _, tc := range []struct {
		name, valueType string
		want            error
	}{
		{"undeclared", "M::Missing", runtime.ErrNotAQuantityValueType},
		{"a calculation", "M::AsTorque", runtime.ErrNotAQuantityValueType},
		{"a feature", "M::e", runtime.ErrNotAQuantityValueType},
		{"a scalar type", "ScalarValues::Real", runtime.ErrNotAQuantityValueType},
		{"the type of every quantity", "Quantities::ScalarQuantityValue", runtime.ErrNotAQuantityValueType},
		{"a measurement unit", "SI::J", runtime.ErrNotAQuantityValueType},
		{"another dimension", "ISQ::MassValue", runtime.ErrTypeMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := protoconv.ProtoToValueIn(joules(tc.valueType), idx, sem); !errors.Is(err, tc.want) {
				t.Fatalf("ProtoToValueIn(value_type %s): err = %v, want %v", tc.valueType, err, tc.want)
			}
		})
	}
	if _, err := protoconv.ProtoToValueIn(joules("ISQ::EnergyValue"), nil, nil); !errors.Is(err, protoconv.ErrQuantityNeedsIndex) {
		t.Fatalf("ProtoToValueIn(value_type without a model): err = %v, want %v", err, protoconv.ErrQuantityNeedsIndex)
	}
	// An alias names the type it stands for.
	for valueType, want := range map[string]string{
		"ISQ::EnergyValue":               "ISQThermodynamics::EnergyValue",
		"ISQThermodynamics::EnergyValue": "ISQThermodynamics::EnergyValue",
		"ISQ::TorqueValue":               "ISQMechanics::TorqueValue",
		"M::Work":                        "M::Work",
	} {
		back, err := protoconv.ProtoToValueIn(joules(valueType), idx, sem)
		if err != nil || back.QuantityKind() == nil || symbols.FQNOf(back.QuantityKind()) != want {
			t.Errorf("ProtoToValueIn(value_type %s) = %v, %v; want it held as %s", valueType, back, err, want)
		}
	}
}

// A quantity a document query projects off a feature crosses held as the
// evaluated feature is, so the sibling type refuses it sent back all the same.
func TestQuantityValueTypeCrossesFromADocumentQuery(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustParse(t, srv, quantityValueTypeWireModel)
	ctx := context.Background()

	resp, err := srv.RunDocumentQuery(ctx, &pb.RunDocumentQueryRequest{
		ModelHash:   modelHash,
		InlineQuery: `Project(WhereType(Descendants(M::rig), "PartUsage"), ("energy", "spare", "derived"))`,
	})
	if err != nil {
		t.Fatalf("RunDocumentQuery failed: %v", err)
	}
	if len(resp.Rows) != 1 || len(resp.Rows[0].Cells) != 3 {
		t.Fatalf("the query answered %v, want one row of three cells", resp.Rows)
	}
	cell := func(i int) *pb.Quantity {
		t.Helper()
		values := resp.Rows[0].Cells[i].Values
		if len(values) != 1 || values[0].GetQuantity() == nil {
			t.Fatalf("cell %d = %v, want one quantity", i, values)
		}
		return values[0].GetQuantity()
	}
	energy, spare, derived := cell(0), cell(1), cell(2)
	for name, quantity := range map[string]*pb.Quantity{"energy": energy, "derived": derived} {
		if got := quantity.GetValueType(); got != "ISQThermodynamics::EnergyValue" {
			t.Errorf("%s crossed with value_type %q, want ISQThermodynamics::EnergyValue", name, got)
		}
	}
	if got := spare.GetValueType(); got != "" {
		t.Errorf("spare crossed with value_type %q, want none", got)
	}
	for _, tc := range []struct {
		quantity *pb.Quantity
		refused  bool
	}{{energy, true}, {derived, true}, {spare, false}} {
		resp, err := srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{
			ModelHash: modelHash, SymbolId: "M::asTorque",
			Arguments: []*pb.Value{{Kind: &pb.Value_Quantity{Quantity: tc.quantity}}},
		})
		if err != nil {
			t.Fatalf("EvaluateCalc: %v", err)
		}
		if strings.Contains(resp.GetError(), "type mismatch") != tc.refused {
			t.Errorf("asTorque(%v): error = %q, want refused = %v", tc.quantity, resp.GetError(), tc.refused)
		}
	}
}
