package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// structuredWireModel yields each structured kind as a feature value and takes a
// vector back as an action input and a calc argument.
const structuredWireModel = `
package S {
  private import ScalarValues::*;
  private import Collections::*;
  private import VectorValues::*;
  private import VectorFunctions::*;
  private import Quantities::*;
  private import SI::*;

  attribute grid : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }
  attribute v : CartesianVectorValue = VectorOf((3.0, 4.0));
  attribute d : VectorQuantityValue = VectorOf((3.0, 4.0)) [m];

  calc def Length { in x : CartesianVectorValue; return : Real = norm(x); }
  calc length : Length;
  calc def Doubled { in x : CartesianVectorValue; return : CartesianVectorValue = cartesianVectorScalarMult(x, 2.0); }
  calc doubled : Doubled;

  action scale {
    in x : CartesianVectorValue;
    out y : CartesianVectorValue;
    first start;
    action inner { assign y := cartesianVectorScalarMult(x, 2.0); }
    then done;
    succession first start then inner;
  }
}
`

func intConst(n int64) runtime.Value {
	return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
}

func realConst(f float64) runtime.Value {
	return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: f}}
}

func intValue(n int64) *pb.Value    { return &pb.Value{Kind: &pb.Value_IntValue{IntValue: n}} }
func realValue(f float64) *pb.Value { return &pb.Value{Kind: &pb.Value_RealValue{RealValue: f}} }

func arrayValue(dimensions []int64, elements ...*pb.Value) *pb.Value {
	return &pb.Value{Kind: &pb.Value_Array{Array: &pb.Array{Dimensions: dimensions, Elements: elements}}}
}

func vectorValue(components ...*pb.Value) *pb.Value {
	return &pb.Value{Kind: &pb.Value_Vector{Vector: &pb.Vector{Components: components}}}
}

func vectorQuantityValue(components ...*pb.Quantity) *pb.Value {
	return &pb.Value{Kind: &pb.Value_VectorQuantity{VectorQuantity: &pb.VectorQuantity{Components: components}}}
}

// mustStructuredModel parses structuredWireModel into srv.
func mustStructuredModel(t *testing.T, srv *Service) (string, *symbols.Index, *semantics.Model) {
	t.Helper()
	resp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: structuredWireModel},
		ContentHash: "structured-wire",
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, diag := range resp.Diagnostics {
		if diag.Severity == "error" {
			t.Fatalf("model has a diagnostic error: %s", diag.Message)
		}
	}
	cached, ok := srv.cache.Get(resp.ModelHash)
	if !ok {
		t.Fatal("parsed model is not cached")
	}
	return resp.ModelHash, cached.Index, NewSymbolContext(cached.Index).Semantics
}

// mustEvaluate evaluates expr and returns the value it produced.
func mustEvaluate(t *testing.T, srv *Service, modelHash, expr string) *pb.Value {
	t.Helper()
	resp, err := srv.Evaluate(context.Background(), &pb.EvaluateRequest{ModelHash: modelHash, Expression: expr})
	if err != nil {
		t.Fatalf("Evaluate(%s): %v", expr, err)
	}
	if resp.Error != "" {
		t.Fatalf("Evaluate(%s): %s", expr, resp.Error)
	}
	return resp.Result
}

// An array crosses as its dimensions and row-major elements at every rank, an
// element that is itself structured included, and reads back as the same array.
func TestArrayRoundTrip(t *testing.T) {
	srv := mustNewService(t, 4)
	_, idx, sem := mustStructuredModel(t, srv)
	metre := mustEvaluateQuantity(t, srv, mustParse(t, srv, quantityModel), "3 [SI::m]")
	metreVal, err := ProtoToQuantity(metre, idx, sem)
	if err != nil {
		t.Fatalf("ProtoToQuantity: %v", err)
	}

	elements := func(n int) []runtime.Value {
		out := make([]runtime.Value, n)
		for i := range out {
			out[i] = intConst(int64(i + 1))
		}
		return out
	}
	inner := runtime.NewArrayValue([]int64{2}, []runtime.Value{metreVal, metreVal})
	cases := []struct {
		name string
		val  runtime.Value
		want string
	}{
		{"rank 0", runtime.NewArrayValue(nil, []runtime.Value{intConst(7)}), "Array()[7]"},
		{"rank 1", runtime.NewArrayValue([]int64{3}, elements(3)), "Array(3)[1, 2, 3]"},
		{"rank 3", runtime.NewArrayValue([]int64{2, 3, 2}, elements(12)), "Array(2, 3, 2)[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12]"},
		{"mixed kinds", runtime.NewArrayValue([]int64{2}, []runtime.Value{realConst(1.5), runtime.NewStringValue("x")}), `Array(2)[1.5, "x"]`},
		{"of quantities", runtime.NewArrayValue([]int64{1, 2}, []runtime.Value{metreVal, metreVal}), "Array(1, 2)[3 [SI::m], 3 [SI::m]]"},
		{"of arrays", runtime.NewArrayValue([]int64{2}, []runtime.Value{inner, inner}), "Array(2)[Array(2)[3 [SI::m], 3 [SI::m]], Array(2)[3 [SI::m], 3 [SI::m]]]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pv := ValueToProto(tc.val, idx)
			if pv.GetArray() == nil {
				t.Fatalf("crossed as %T: %v", pv.GetKind(), pv)
			}
			if got, want := len(pv.GetArray().GetDimensions()), tc.val.Array().Rank(); got != want {
				t.Errorf("%d dimensions, want %d", got, want)
			}
			back, err := ProtoToValueIn(pv, idx, sem)
			if err != nil {
				t.Fatalf("ProtoToValueIn: %v", err)
			}
			if back.Kind != runtime.ValArray {
				t.Fatalf("read back as %s", back.Kind)
			}
			if got := runtime.FormatValue(back); got != tc.want {
				t.Errorf("round trip = %s, want %s", got, tc.want)
			}
			if got := runtime.FormatValue(tc.val); got != tc.want {
				t.Errorf("sent = %s, want %s", got, tc.want)
			}
		})
	}

	// A quantity element keeps its reduction over the model's own base units.
	back, err := ProtoToValueIn(ValueToProto(inner, idx), idx, sem)
	if err != nil {
		t.Fatalf("ProtoToValueIn: %v", err)
	}
	if !back.Array().Elements[0].Quantity().Unit.Term.Commensurable(metreVal.Quantity().Unit.Term) {
		t.Error("a quantity element lost its reduction on the way back")
	}
}

// mustParse parses content into srv and returns its model hash.
func mustParse(t *testing.T, srv *Service, content string) string {
	t.Helper()
	resp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{Source: &pb.ParseFileRequest_Content{Content: content}})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	return resp.ModelHash
}

// A vector crosses component by component, Integer and Real kept apart, and
// reads back as the same vector.
func TestVectorRoundTrip(t *testing.T) {
	idx := symbols.NewIndex()
	cases := []struct {
		name  string
		val   runtime.Value
		kinds []semantics.ValueKind
		want  string
	}{
		{"integers", runtime.NewVectorValue([]semantics.Value{intConst(1).Const, intConst(2).Const}), []semantics.ValueKind{semantics.ValInt, semantics.ValInt}, "⟨1, 2⟩"},
		{"reals", runtime.NewVectorValue([]semantics.Value{realConst(3).Const, realConst(4).Const}), []semantics.ValueKind{semantics.ValReal, semantics.ValReal}, "⟨3.0, 4.0⟩"},
		{"mixed", runtime.NewVectorValue([]semantics.Value{intConst(1).Const, realConst(2.5).Const}), []semantics.ValueKind{semantics.ValInt, semantics.ValReal}, "⟨1, 2.5⟩"},
		{"empty", runtime.NewVectorValue(nil), nil, "⟨⟩"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pv := ValueToProto(tc.val, idx)
			if pv.GetVector() == nil {
				t.Fatalf("crossed as %T: %v", pv.GetKind(), pv)
			}
			if got, want := len(pv.GetVector().GetComponents()), tc.val.Vector().Dimension(); got != want {
				t.Errorf("%d components, want %d", got, want)
			}
			back, err := ProtoToValueIn(pv, idx, nil)
			if err != nil {
				t.Fatalf("ProtoToValueIn: %v", err)
			}
			if back.Kind != runtime.ValVector {
				t.Fatalf("read back as %s", back.Kind)
			}
			for i, kind := range tc.kinds {
				if got := back.Vector().Elements[i].Kind; got != kind {
					t.Errorf("component %d read back as %v, want %v", i+1, got, kind)
				}
			}
			if got := runtime.FormatValue(back); got != tc.want {
				t.Errorf("round trip = %s, want %s", got, tc.want)
			}
		})
	}
}

// A vector quantity crosses as one Quantity per axis, unit text and reduction
// intact, in a composed unit and with axes in differing units alike.
func TestVectorQuantityRoundTrip(t *testing.T) {
	srv, modelHash, idx, sem := mustQuantityModel(t)
	quantity := func(expr string) *runtime.Quantity {
		val, err := ProtoToQuantity(mustEvaluateQuantity(t, srv, modelHash, expr), idx, sem)
		if err != nil {
			t.Fatalf("ProtoToQuantity(%s): %v", expr, err)
		}
		return val.Quantity()
	}
	speed := quantity("10.0 [SI::m] / 2.0 [SI::s]")
	metre := quantity("3 [SI::m]")
	kilogram := quantity("5.0 [SI::kg]")

	cases := []struct {
		name string
		val  runtime.Value
		want string
	}{
		{"composed unit", runtime.NewVectorQuantityValue(
			[]semantics.Value{realConst(3).Const, realConst(4).Const},
			[]runtime.Unit{speed.Unit, speed.Unit}), "⟨3.0, 4.0⟩ [SI::m/SI::s]"},
		{"integer magnitudes", runtime.NewVectorQuantityValue(
			[]semantics.Value{intConst(3).Const, intConst(4).Const},
			[]runtime.Unit{metre.Unit, metre.Unit}), "⟨3, 4⟩ [SI::m]"},
		{"differing units", runtime.NewVectorQuantityValue(
			[]semantics.Value{realConst(1).Const, realConst(2).Const},
			[]runtime.Unit{metre.Unit, kilogram.Unit}), "⟨1.0 [SI::m], 2.0 [SI::kg]⟩"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pv := ValueToProto(tc.val, idx)
			if pv.GetVectorQuantity() == nil {
				t.Fatalf("crossed as %T: %v", pv.GetKind(), pv)
			}
			sent := tc.val.VectorQuantity()
			for i, comp := range pv.GetVectorQuantity().GetComponents() {
				if comp.GetUnit() != sent.Units[i].Text {
					t.Errorf("component %d unit = %q, want %q", i+1, comp.GetUnit(), sent.Units[i].Text)
				}
				if comp.GetUnitTerm() == nil {
					t.Errorf("component %d crossed without its reduction", i+1)
				}
			}
			back, err := ProtoToValueIn(pv, idx, sem)
			if err != nil {
				t.Fatalf("ProtoToValueIn: %v", err)
			}
			if back.Kind != runtime.ValVectorQuantity {
				t.Fatalf("read back as %s", back.Kind)
			}
			got := back.VectorQuantity()
			for i := range sent.Num {
				if got.Num[i] != sent.Num[i] {
					t.Errorf("component %d = %v, want %v", i+1, got.Num[i], sent.Num[i])
				}
				if !got.Units[i].Term.Same(sent.Units[i].Term) {
					t.Errorf("component %d reduction = %v, want %v", i+1, got.Units[i].Term, sent.Units[i].Term)
				}
			}
			if s := runtime.FormatValue(back); s != tc.want {
				t.Errorf("round trip = %s, want %s", s, tc.want)
			}
		})
	}
}

// A malformed structured value is refused with a typed error naming what is
// wrong, never read under another shape or as another number.
func TestMalformedStructuredValuesAreRejected(t *testing.T) {
	srv, modelHash, idx, sem := mustQuantityModel(t)
	metre := mustEvaluateQuantity(t, srv, modelHash, "3 [SI::m]")
	unreduced := &pb.Quantity{Magnitude: &pb.Quantity_IntMagnitude{IntMagnitude: 3}, Unit: "SI::m"}

	cases := []struct {
		name string
		val  *pb.Value
		want error
	}{
		{"too few elements", arrayValue([]int64{2, 3}, intValue(1), intValue(2)), ErrArrayShapeMismatch},
		{"too many elements", arrayValue([]int64{2}, intValue(1), intValue(2), intValue(3)), ErrArrayShapeMismatch},
		{"rank 0 with two elements", arrayValue(nil, intValue(1), intValue(2)), ErrArrayShapeMismatch},
		{"zero dimension", arrayValue([]int64{0}), ErrArrayDimensionNotPositive},
		{"negative dimension", arrayValue([]int64{-1, 2}, intValue(1), intValue(2)), ErrArrayDimensionNotPositive},
		{"overflowing dimensions", arrayValue([]int64{1 << 62, 4}), ErrArrayShapeMismatch},
		{"nested malformed array", arrayValue([]int64{1}, arrayValue([]int64{2}, intValue(1))), ErrArrayShapeMismatch},
		{"array of unset", arrayValue([]int64{1}, &pb.Value{Kind: &pb.Value_Unset{Unset: true}}), ErrUnsetNotAccepted},
		{"string component", vectorValue(realValue(1), &pb.Value{Kind: &pb.Value_StringValue{StringValue: "2"}}), ErrVectorComponentNotNumeric},
		{"bool component", vectorValue(&pb.Value{Kind: &pb.Value_BoolValue{BoolValue: true}}), ErrVectorComponentNotNumeric},
		{"quantity component", vectorValue(&pb.Value{Kind: &pb.Value_Quantity{Quantity: metre}}), ErrVectorComponentNotNumeric},
		{"nested vector component", vectorValue(vectorValue(intValue(1))), ErrVectorComponentNotNumeric},
		{"empty component", vectorValue(&pb.Value{}), ErrVectorComponentNotNumeric},
		{"empty vector quantity", vectorQuantityValue(), ErrVectorQuantityEmpty},
		{"unreduced unit", vectorQuantityValue(metre, unreduced), ErrUnitNotReduced},
		{"unknown base unit", vectorQuantityValue(&pb.Quantity{
			Magnitude: &pb.Quantity_RealMagnitude{RealMagnitude: 1}, Unit: "furlong",
			UnitTerm: &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1, Factors: []*pb.UnitFactor{{UnitId: "SI::furlong", Exponent: 1}}},
		}), ErrUnknownBaseUnit},
		{"component without quantity", vectorQuantityValue(metre, nil), ErrVectorComponentNotNumeric},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			val, err := ProtoToValueIn(tc.val, idx, sem)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ProtoToValueIn = %v, %v; want %v", val, err, tc.want)
			}
			if val.Kind != runtime.ValInvalid {
				t.Errorf("a rejected value was still returned: %v", val)
			}
		})
	}

	// A quantity component's reduction needs the model, as a scalar's does.
	if _, err := ProtoToValueIn(vectorQuantityValue(metre), nil, nil); !errors.Is(err, ErrQuantityNeedsIndex) {
		t.Errorf("without an index: err = %v, want %v", err, ErrQuantityNeedsIndex)
	}
}

// Evaluate, Instantiate, ExecuteAction and EvaluateCalc all carry the structured
// kinds as themselves, in both directions.
func TestStructuredValuesCrossEveryValueSurface(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 4)
	modelHash, _, _ := mustStructuredModel(t, srv)

	grid := mustEvaluate(t, srv, modelHash, "S::grid")
	if grid.GetArray() == nil || len(grid.GetArray().GetDimensions()) != 2 || len(grid.GetArray().GetElements()) != 6 {
		t.Fatalf("S::grid = %v, want an array of dimensions (2, 3) and 6 elements", grid)
	}
	if dims := grid.GetArray().GetDimensions(); dims[0] != 2 || dims[1] != 3 {
		t.Errorf("dimensions = %v, want [2 3]", dims)
	}
	for i, elem := range grid.GetArray().GetElements() {
		if elem.GetIntValue() != int64(i+1) {
			t.Errorf("element %d = %v, want %d", i, elem, i+1)
		}
	}

	v := mustEvaluate(t, srv, modelHash, "S::v")
	if v.GetVector() == nil || len(v.GetVector().GetComponents()) != 2 {
		t.Fatalf("S::v = %v, want a vector of two components", v)
	}
	if c := v.GetVector().GetComponents(); c[0].GetRealValue() != 3 || c[1].GetRealValue() != 4 {
		t.Errorf("components = %v, want 3.0 and 4.0", c)
	}

	d := mustEvaluate(t, srv, modelHash, "S::d")
	if d.GetVectorQuantity() == nil || len(d.GetVectorQuantity().GetComponents()) != 2 {
		t.Fatalf("S::d = %v, want a vector quantity of two components", d)
	}
	for i, want := range []float64{3, 4} {
		comp := d.GetVectorQuantity().GetComponents()[i]
		if comp.GetRealMagnitude() != want || comp.GetUnit() != "m" || describeUnitTerm(comp.GetUnitTerm()) != "SI::metre" {
			t.Errorf("component %d = %v, want %v [m] reducing to SI::metre", i+1, comp, want)
		}
	}

	inst, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: modelHash, SymbolId: "S"})
	if err != nil || inst.Error != "" {
		t.Fatalf("Instantiate: err = %v, error = %q", err, inst.GetError())
	}
	for name, has := range map[string]func(*pb.Value) bool{
		"v": func(pv *pb.Value) bool { return pv.GetVector() != nil },
		"d": func(pv *pb.Value) bool { return pv.GetVectorQuantity() != nil },
	} {
		fv := inst.Instance.FeatureValues[name]
		if fv == nil || fv.Error != "" || !has(fv.Value) {
			t.Errorf("feature value %s = %v, want the structured arm", name, fv)
		}
	}

	sent := vectorValue(realValue(3), realValue(4))
	act, err := srv.ExecuteAction(ctx, &pb.ExecuteActionRequest{
		ModelHash:      modelHash,
		ActionSymbolId: "S::scale",
		Inputs:         map[string]*pb.Value{"x": sent},
	})
	if err != nil || act.Error != "" {
		t.Fatalf("ExecuteAction: err = %v, error = %q", err, act.GetError())
	}
	if y := act.Outputs["y"].GetVector(); y == nil || y.Components[0].GetRealValue() != 6 || y.Components[1].GetRealValue() != 8 {
		t.Errorf("output y = %v, want ⟨6.0, 8.0⟩", act.Outputs["y"])
	}

	calc, err := srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "S::length", Arguments: []*pb.Value{sent}})
	if err != nil || calc.Error != "" {
		t.Fatalf("EvaluateCalc(length): err = %v, error = %q", err, calc.GetError())
	}
	if calc.Result.GetRealValue() != 5 {
		t.Errorf("length(⟨3.0, 4.0⟩) = %v, want 5.0", calc.Result)
	}
	calc, err = srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "S::doubled", Arguments: []*pb.Value{sent}})
	if err != nil || calc.Error != "" {
		t.Fatalf("EvaluateCalc(doubled): err = %v, error = %q", err, calc.GetError())
	}
	if got := calc.Result.GetVector(); got == nil || got.Components[0].GetRealValue() != 6 || got.Components[1].GetRealValue() != 8 {
		t.Errorf("doubled(⟨3.0, 4.0⟩) = %v, want ⟨6.0, 8.0⟩", calc.Result)
	}

	// A malformed argument is an in-band error, as a malformed quantity is.
	calc, err = srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "S::length", Arguments: []*pb.Value{
		vectorValue(realValue(3), &pb.Value{Kind: &pb.Value_StringValue{StringValue: "4"}}),
	}})
	if err != nil {
		t.Fatalf("EvaluateCalc(malformed): %v", err)
	}
	if !strings.Contains(calc.Error, ErrVectorComponentNotNumeric.Error()) {
		t.Errorf("EvaluateCalc(malformed) error = %q, want one naming %v", calc.Error, ErrVectorComponentNotNumeric)
	}
}

// The structured arms are advertised, so a client can require them, and a
// service withholding them names each value as unsupported rather than
// flattening it.
func TestStructuredValuesCapability(t *testing.T) {
	found := false
	for _, c := range Capabilities() {
		found = found || c == CapabilityStructuredValues
	}
	if !found {
		t.Errorf("capabilities %v do not include %q", Capabilities(), CapabilityStructuredValues)
	}

	withheld := mustNewServiceWithout(t, CapabilityStructuredValues)
	modelHash, _, _ := mustStructuredModel(t, withheld)
	for expr, want := range map[string]string{
		"S::grid": "unsupported: array Array(2, 3)[1, 2, 3, 4, 5, 6]",
		"S::v":    "unsupported: vector ⟨3.0, 4.0⟩",
		"S::d":    "unsupported: vector quantity ⟨3.0, 4.0⟩ [m]",
	} {
		got := mustEvaluate(t, withheld, modelHash, expr)
		if got.GetSequence() != nil {
			t.Errorf("%s crossed as a sequence without %s: %v", expr, CapabilityStructuredValues, got)
		}
		if got.GetNull() != want {
			t.Errorf("%s without %s = %v, want null %q", expr, CapabilityStructuredValues, got, want)
		}
	}

	// An array's elements are filtered like any values when the arm itself crosses.
	served := mustNewServiceWithout(t, CapabilityComplexValues)
	pv := arrayValue([]int64{1}, &pb.Value{Kind: &pb.Value_Complex{Complex: ComplexToProto(complex(0, 1))}})
	served.filterValueCapabilities(pv)
	if pv.GetArray() == nil || !strings.Contains(pv.GetArray().GetElements()[0].GetNull(), "complex number") {
		t.Errorf("array of a complex without complex_values = %v, want the element withheld", pv)
	}
}

func TestValueCarriesStructured(t *testing.T) {
	one := intValue(1)
	sequence := func(elements ...*pb.Value) *pb.Value {
		return &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: elements}}}
	}
	for _, tc := range []struct {
		name  string
		value *pb.Value
		want  bool
	}{
		{"nil", nil, false},
		{"int", one, false},
		{"array", arrayValue([]int64{1}, one), true},
		{"vector", vectorValue(one), true},
		{"vector quantity", vectorQuantityValue(), true},
		{"sequence of ints", sequence(one, one), false},
		{"sequence with an array", sequence(one, sequence(arrayValue([]int64{1}, one))), true},
	} {
		if got := ValueCarriesStructured(tc.value); got != tc.want {
			t.Errorf("ValueCarriesStructured(%s) = %v, want %v", tc.name, got, tc.want)
		}
	}
	z := &pb.Value{Kind: &pb.Value_Complex{Complex: ComplexToProto(complex(1, 2))}}
	if !ValueCarriesComplex(arrayValue([]int64{1}, z)) {
		t.Error("ValueCarriesComplex misses a complex element of an array")
	}
}

// A service without structured_values refuses a structured input outright,
// however deeply nested, rather than reading it as another value.
func TestStructuredInputNeedsStructuredValues(t *testing.T) {
	ctx := context.Background()
	withheld := mustNewServiceWithout(t, CapabilityStructuredValues)
	modelHash, _, _ := mustStructuredModel(t, withheld)
	vector := vectorValue(realValue(3), realValue(4))
	nested := &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: []*pb.Value{intValue(1), vector}}}}
	for name, input := range map[string]*pb.Value{"vector": vector, "nested": nested, "array": arrayValue([]int64{1}, intValue(1))} {
		_, err := withheld.ExecuteAction(ctx, &pb.ExecuteActionRequest{
			ModelHash:      modelHash,
			ActionSymbolId: "S::scale",
			Inputs:         map[string]*pb.Value{"x": input},
		})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityStructuredValues) {
			t.Errorf("ExecuteAction with %s input without structured_values: err = %v, want UNIMPLEMENTED naming the capability", name, err)
		}
		_, err = withheld.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "S::length", Arguments: []*pb.Value{input}})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityStructuredValues) {
			t.Errorf("EvaluateCalc with %s argument without structured_values: err = %v, want UNIMPLEMENTED naming the capability", name, err)
		}
	}
}

// The wire Value has no tensor arm, so a tensor quantity crosses as an unsupported
// null naming it, never flattened to a sequence, an array or one quantity.
func TestTensorQuantityCrossesAsUnsupported(t *testing.T) {
	metre := semantics.Unit{
		Text:    "m",
		Product: semantics.OpaqueUnitProduct("m", semantics.UnitTerm{Scale: semantics.UnitScale(1)}),
	}
	num := make([]semantics.Value, 4)
	units := make([]runtime.Unit, 4)
	for i := range num {
		num[i] = semantics.Value{Kind: semantics.ValReal, Real: float64(i + 1)}
		units[i] = metre
	}
	pv := ValueToProto(runtime.NewTensorQuantityValue([]int64{2, 2}, num, units), nil)
	if pv.GetSequence() != nil || pv.GetArray() != nil || pv.GetQuantity() != nil || pv.GetVector() != nil {
		t.Fatalf("a tensor quantity crossed as %T: %v", pv.GetKind(), pv)
	}
	if got, want := pv.GetNull(), "unsupported: tensor quantity Tensor(2, 2)[1.0, 2.0, 3.0, 4.0] [m]"; got != want {
		t.Errorf("ValueToProto(tensor) = %v, want null %q", pv, want)
	}
}

// The wire Value has no frame arm either: a coordinate frame and a transformation
// cross as unsupported nulls naming them, never as sequences of their axes.
func TestCoordinateFrameCrossesAsUnsupported(t *testing.T) {
	metre := semantics.Unit{Text: "m", Product: semantics.OpaqueUnitProduct("m", semantics.UnitTerm{Scale: semantics.UnitScale(1)})}
	frame := &runtime.CoordinateFrame{Dimensions: []int64{3}, Axes: []runtime.Unit{metre, metre, metre}, Text: "spatialCF"}
	for want, val := range map[string]runtime.Value{
		"unsupported: coordinate frame spatialCF [m, m, m]":                                          runtime.NewCoordinateFrameValue(frame),
		"unsupported: coordinate transformation a coordinate transformation (spatialCF → spatialCF)": runtime.NewCoordinateTransformationValue(&runtime.CoordinateTransformation{Source: frame, Target: frame}),
	} {
		pv := ValueToProto(val, nil)
		if pv.GetSequence() != nil || pv.GetQuantity() != nil || pv.GetStringValue() != "" {
			t.Fatalf("%s crossed as %T: %v", runtime.FormatValue(val), pv.GetKind(), pv)
		}
		if pv.GetNull() != want {
			t.Errorf("ValueToProto(%s) = %v, want null %q", runtime.FormatValue(val), pv, want)
		}
	}
}
