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
)

// setTensorWireModel yields a set and tensors of rank two and three as feature
// values, and takes each back as a calc argument that reads it.
const setTensorWireModel = `
package W {
  private import ScalarValues::*;
  private import Collections::*;
  private import CollectionFunctions::*;
  private import Quantities::*;
  private import MeasurementReferences::*;
  private import SI::*;
  private import TensorCalculations::*;

  attribute s : Set { :>> elements = (3, 1, 2, 2, 3); }
  attribute e : Set { :>> elements = (); }
  attribute mixed : Set { :>> elements = ("b", 2, true, 1.5, "a", 3 [m]); }

  attribute cubeRef : TensorMeasurementReference {
    :>> dimensions = (2, 2, 2);
    :>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa);
  }
  attribute planeRef : TensorMeasurementReference {
    :>> dimensions = (2, 2);
    :>> mRefs = (m, m, m, m);
  }
  attribute cube : TensorQuantityValue = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef);
  attribute plane : TensorQuantityValue = TensorCalculations::'['((1, 2, 3, 4), planeRef);
  attribute tensors : TensorQuantityValue[*] ordered = (cube, plane);

  calc def SizeOf { in c : Integer[0..*]; return : Natural = SequenceFunctions::size(c); }
  calc sizeOf : SizeOf;
  calc def Corner { in t : TensorQuantityValue; return : ScalarQuantityValue = t#(2, 2, 2); }
  calc corner : Corner;
  calc def Rank { in t : TensorQuantityValue; return : Natural = t.order; }
  calc rank : Rank;
}
`

func mustSetTensorModel(t *testing.T, srv *Service) string {
	t.Helper()
	return mustParse(t, srv, setTensorWireModel)
}

func setOf(elements ...*pb.Value) *pb.Value {
	return &pb.Value{Kind: &pb.Value_Set{Set: &pb.ValueSet{Elements: elements}}}
}

func tensorQuantityValue(dimensions []int64, components ...*pb.Quantity) *pb.Value {
	return &pb.Value{Kind: &pb.Value_TensorQuantity{TensorQuantity: &pb.TensorQuantity{Dimensions: dimensions, Components: components}}}
}

func stringValue(s string) *pb.Value { return &pb.Value{Kind: &pb.Value_StringValue{StringValue: s}} }

// A set crosses as its own arm holding each distinct element once, in the
// runtime's canonical order, and reads back as an equal set whatever order it
// is sent in.
func TestSetRoundTrip(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustSetTensorModel(t, srv)
	cached, ok := srv.cache.Get(modelHash)
	if !ok {
		t.Fatal("parsed model is not cached")
	}
	idx, sem := cached.Index, NewSymbolContext(cached.Index).Semantics

	cases := []struct {
		expr string
		want string
	}{
		{"W::s.elements", "Set{1, 2, 3}"},
		{"W::e.elements", "Set{}"},
		{"W::mixed.elements", `Set{true, 1.5, 2, "a", "b", 3 [m]}`},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			pv := mustEvaluate(t, srv, modelHash, tc.expr)
			if pv.GetSet() == nil {
				t.Fatalf("%s crossed as %T: %v", tc.expr, pv.GetKind(), pv)
			}
			shown := displayValue(pv)
			if got := runtime.FormatValue(shown); got != tc.want {
				t.Errorf("%s crossed as %s, want %s", tc.expr, got, tc.want)
			}
			// The elements are sent in canonical order, not the order written.
			var sent []string
			for _, elem := range pv.GetSet().GetElements() {
				sent = append(sent, runtime.FormatValue(displayValue(elem)))
			}
			if got := "Set{" + strings.Join(sent, ", ") + "}"; got != tc.want {
				t.Errorf("%s elements on the wire in order %s, want %s", tc.expr, got, tc.want)
			}

			back, err := ProtoToValueIn(pv, idx, sem)
			if err != nil {
				t.Fatalf("ProtoToValueIn: %v", err)
			}
			if back.Kind != runtime.ValSet {
				t.Fatalf("read back as %s", back.Kind)
			}
			if got := runtime.FormatValue(back); got != tc.want {
				t.Errorf("round trip = %s, want %s", got, tc.want)
			}
		})
	}

	// A client may send the elements in any order: the set read is the same.
	written := setOf(intValue(3), intValue(1), intValue(2))
	canonical := setOf(intValue(1), intValue(2), intValue(3))
	a, err := ProtoToValueIn(written, idx, sem)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ProtoToValueIn(canonical, idx, sem)
	if err != nil {
		t.Fatal(err)
	}
	if !a.Set().Equal(b.Set()) || a.Set().Size() != 3 {
		t.Errorf("sets sent in two orders read back as %s and %s", runtime.FormatValue(a), runtime.FormatValue(b))
	}

	// The empty set, a set of sets and a set nested in a sequence read back as
	// themselves, the nested sets deduplicated by set equality.
	empty, err := ProtoToValueIn(setOf(), idx, sem)
	if err != nil || empty.Kind != runtime.ValSet || empty.Set().Size() != 0 {
		t.Errorf("empty set read back as %v, %v", empty, err)
	}
	nested := setOf(written, setOf())
	back, err := ProtoToValueIn(nested, idx, sem)
	if err != nil {
		t.Fatal(err)
	}
	if got := runtime.FormatValue(back); got != "Set{Set{1, 2, 3}, Set{}}" {
		t.Errorf("set of sets read back as %s", got)
	}
	if got := runtime.FormatValue(displayValue(ValueToProto(back, idx))); got != "Set{Set{1, 2, 3}, Set{}}" {
		t.Errorf("set of sets crossed as %s", got)
	}
	seq := &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: []*pb.Value{written, intValue(4)}}}}
	back, err = ProtoToValueIn(seq, idx, sem)
	if err != nil {
		t.Fatal(err)
	}
	if got := runtime.FormatValue(back); got != "[Set{1, 2, 3}, 4]" {
		t.Errorf("sequence holding a set read back as %s", got)
	}
}

// A set sent with an element twice is refused with a typed error rather than
// read as the set holding it once, so a sequence sent under the wrong arm is
// never silently deduplicated; a malformed element is refused as it is anywhere.
func TestMalformedSetsAreRejected(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustSetTensorModel(t, srv)
	cached, _ := srv.cache.Get(modelHash)
	idx, sem := cached.Index, NewSymbolContext(cached.Index).Semantics

	cases := []struct {
		name string
		val  *pb.Value
		want error
	}{
		{"repeated integer", setOf(intValue(1), intValue(2), intValue(1)), ErrSetElementRepeated},
		{"repeated string", setOf(stringValue("a"), stringValue("a")), ErrSetElementRepeated},
		{"repeated nested set", setOf(setOf(intValue(1)), setOf(intValue(1))), ErrSetElementRepeated},
		{"nested sets equal in another order", setOf(setOf(intValue(1), intValue(2)), setOf(intValue(2), intValue(1))), ErrSetElementRepeated},
		{"Integer and the equal Real", setOf(intValue(1), realValue(1)), ErrSetElementRepeated},
		{"unset element", setOf(&pb.Value{Kind: &pb.Value_Unset{Unset: true}}), ErrUnsetNotAccepted},
		{"malformed element", setOf(arrayValue([]int64{0})), ErrArrayDimensionNotPositive},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ProtoToValueIn(tc.val, idx, sem)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ProtoToValueIn = %v, want %v", err, tc.want)
			}
		})
	}
}

// A tensor quantity crosses at any rank as its dimensions and one Quantity per
// row-major component, unit and reduction included, and reads back as the same
// tensor; a rank-two tensor and a rank-one tensor are not a Vector or a
// VectorQuantity.
func TestTensorQuantityRoundTrip(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustSetTensorModel(t, srv)
	cached, _ := srv.cache.Get(modelHash)
	idx, sem := cached.Index, NewSymbolContext(cached.Index).Semantics

	cases := []struct {
		expr string
		dims []int64
		want string
	}{
		{"W::cube", []int64{2, 2, 2}, "Tensor(2, 2, 2)[1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0] [Pa]"},
		{"W::plane", []int64{2, 2}, "Tensor(2, 2)[1, 2, 3, 4] [m]"},
		{"W::cube + W::cube", []int64{2, 2, 2}, "Tensor(2, 2, 2)[2.0, 4.0, 6.0, 8.0, 10.0, 12.0, 14.0, 16.0] [Pa]"},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			pv := mustEvaluate(t, srv, modelHash, tc.expr)
			ptq := pv.GetTensorQuantity()
			if ptq == nil {
				t.Fatalf("%s crossed as %T: %v", tc.expr, pv.GetKind(), pv)
			}
			if len(ptq.GetDimensions()) != len(tc.dims) {
				t.Fatalf("%s crossed with dimensions %v, want %v", tc.expr, ptq.GetDimensions(), tc.dims)
			}
			for i, d := range tc.dims {
				if ptq.GetDimensions()[i] != d {
					t.Errorf("%s dimension %d = %d, want %d", tc.expr, i+1, ptq.GetDimensions()[i], d)
				}
			}
			for i, comp := range ptq.GetComponents() {
				if comp.GetUnitTerm() == nil || comp.GetUnit() == "" {
					t.Errorf("component %d crossed as %v, want a unit with its reduction", i+1, comp)
				}
			}
			if got := runtime.FormatValue(displayValue(pv)); got != tc.want {
				t.Errorf("%s crossed as %s, want %s", tc.expr, got, tc.want)
			}

			back, err := ProtoToValueIn(pv, idx, sem)
			if err != nil {
				t.Fatalf("ProtoToValueIn: %v", err)
			}
			if back.Kind != runtime.ValTensorQuantity {
				t.Fatalf("read back as %s", back.Kind)
			}
			if got := runtime.FormatValue(back); got != tc.want {
				t.Errorf("round trip = %s, want %s", got, tc.want)
			}
			sent := mustEvaluateTensor(t, srv, modelHash, tc.expr)
			for i := range sent.Units {
				if !back.TensorQuantity().Units[i].Term.Same(sent.Units[i].Term) {
					t.Errorf("component %d reduction = %v, want %v", i+1, back.TensorQuantity().Units[i].Term, sent.Units[i].Term)
				}
			}
		})
	}

	// A tensor of rank one is its own kind, as it is in the runtime.
	metre := mustEvaluateQuantity(t, srv, modelHash, "3 [SI::m]")
	line, err := ProtoToValueIn(tensorQuantityValue([]int64{2}, metre, metre), idx, sem)
	if err != nil {
		t.Fatal(err)
	}
	if line.Kind != runtime.ValTensorQuantity || runtime.FormatValue(line) != "Tensor(2)[3, 3] [SI::m]" {
		t.Errorf("rank-one tensor read back as %s %s", line.Kind, runtime.FormatValue(line))
	}
	if pv := ValueToProto(line, idx); pv.GetTensorQuantity() == nil {
		t.Errorf("rank-one tensor crossed as %T", pv.GetKind())
	}

	// A tensor with no magnitude in a component still has no wire form.
	num := []semantics.Value{{Kind: semantics.ValInt, Int: 1}, {}}
	units := []runtime.Unit{line.TensorQuantity().Units[0], line.TensorQuantity().Units[0]}
	pv := ValueToProto(runtime.NewTensorQuantityValue([]int64{2}, num, units), idx)
	if pv.GetNull() != "unsupported: tensor quantity with a non-numeric component" {
		t.Errorf("tensor with a non-numeric component crossed as %v", pv)
	}
}

// mustEvaluateTensor evaluates expr through the runtime and returns the tensor.
func mustEvaluateTensor(t *testing.T, srv *Service, modelHash, expr string) *runtime.TensorQuantity {
	t.Helper()
	cached, _ := srv.cache.Get(modelHash)
	idx, sem := cached.Index, NewSymbolContext(cached.Index).Semantics
	val, err := ProtoToValueIn(mustEvaluate(t, srv, modelHash, expr), idx, sem)
	if err != nil || val.Kind != runtime.ValTensorQuantity {
		t.Fatalf("%s = %v, %v; want a tensor", expr, val, err)
	}
	return val.TensorQuantity()
}

// A malformed tensor is refused with a typed error naming what is wrong, never
// read under another shape or with a unit it did not send.
func TestMalformedTensorQuantitiesAreRejected(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustSetTensorModel(t, srv)
	cached, _ := srv.cache.Get(modelHash)
	idx, sem := cached.Index, NewSymbolContext(cached.Index).Semantics
	metre := mustEvaluateQuantity(t, srv, modelHash, "3 [SI::m]")
	unreduced := &pb.Quantity{Magnitude: &pb.Quantity_IntMagnitude{IntMagnitude: 3}, Unit: "SI::m"}

	cases := []struct {
		name string
		val  *pb.Value
		want error
	}{
		{"too few components", tensorQuantityValue([]int64{2, 2, 2}, metre, metre, metre, metre, metre, metre, metre), ErrTensorShapeMismatch},
		{"too many components", tensorQuantityValue([]int64{2, 2}, metre, metre, metre, metre, metre), ErrTensorShapeMismatch},
		{"rank 0 without its component", tensorQuantityValue(nil), ErrTensorShapeMismatch},
		{"zero dimension", tensorQuantityValue([]int64{0, 2}), ErrTensorDimensionNotPositive},
		{"negative dimension", tensorQuantityValue([]int64{2, -1}), ErrTensorDimensionNotPositive},
		{"overflowing shape", tensorQuantityValue([]int64{1 << 40, 1 << 40}, metre), ErrTensorShapeMismatch},
		{"unreduced unit", tensorQuantityValue([]int64{1}, unreduced), ErrUnitNotReduced},
		{"missing component", tensorQuantityValue([]int64{1}, nil), ErrTensorComponentMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ProtoToValueIn(tc.val, idx, sem)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ProtoToValueIn = %v, want %v", err, tc.want)
			}
		})
	}
}

// Sets and tensors cross every surface a value does: a feature value, a
// sequence element, a calc argument the model reads as the kind it is.
func TestSetAndTensorCrossEveryValueSurface(t *testing.T) {
	ctx := context.Background()
	srv := mustNewService(t, 4)
	modelHash := mustSetTensorModel(t, srv)

	inst, err := srv.Instantiate(ctx, &pb.InstantiateRequest{ModelHash: modelHash, SymbolId: "W"})
	if err != nil || inst.Error != "" {
		t.Fatalf("Instantiate: err = %v, error = %q", err, inst.GetError())
	}
	if fv := inst.Instance.FeatureValues["cube"]; fv == nil || fv.Error != "" || fv.Value.GetTensorQuantity() == nil {
		t.Errorf("feature value cube = %v, want a tensor quantity", fv)
	}
	tensors := mustEvaluate(t, srv, modelHash, "W::tensors")
	elems := tensors.GetSequence().GetElements()
	if len(elems) != 2 || elems[0].GetTensorQuantity() == nil || elems[1].GetTensorQuantity() == nil {
		t.Fatalf("W::tensors = %v, want a sequence of two tensors", tensors)
	}

	cube := mustEvaluate(t, srv, modelHash, "W::cube")
	calc, err := srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "W::corner", Arguments: []*pb.Value{cube}})
	if err != nil || calc.Error != "" {
		t.Fatalf("EvaluateCalc(corner): err = %v, error = %q", err, calc.GetError())
	}
	if got := calc.Result.GetQuantity(); got == nil || got.GetRealMagnitude() != 8 || got.GetUnit() != "Pa" {
		t.Errorf("corner(cube) = %v, want 8.0 [Pa]", calc.Result)
	}
	calc, err = srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "W::rank", Arguments: []*pb.Value{cube}})
	if err != nil || calc.Error != "" {
		t.Fatalf("EvaluateCalc(rank): err = %v, error = %q", err, calc.GetError())
	}
	if calc.Result.GetIntValue() != 3 {
		t.Errorf("rank(cube) = %v, want 3", calc.Result)
	}

	// A set sent for a nonunique parameter is read as the sequence of its
	// elements, in whatever order a client sends them.
	calc, err = srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "W::sizeOf", Arguments: []*pb.Value{
		setOf(intValue(3), intValue(1), intValue(2)),
	}})
	if err != nil || calc.Error != "" {
		t.Fatalf("EvaluateCalc(sizeOf): err = %v, error = %q", err, calc.GetError())
	}
	if calc.Result.GetIntValue() != 3 {
		t.Errorf("sizeOf({3, 1, 2}) = %v, want 3", calc.Result)
	}

	// A malformed argument is an in-band error, as a malformed quantity is.
	calc, err = srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "W::sizeOf", Arguments: []*pb.Value{
		setOf(intValue(1), intValue(1)),
	}})
	if err != nil {
		t.Fatalf("EvaluateCalc(repeated): %v", err)
	}
	if !strings.Contains(calc.Error, ErrSetElementRepeated.Error()) {
		t.Errorf("EvaluateCalc(repeated) error = %q, want one naming %v", calc.Error, ErrSetElementRepeated)
	}
	calc, err = srv.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "W::rank", Arguments: []*pb.Value{
		tensorQuantityValue([]int64{2}, mustEvaluateQuantity(t, srv, modelHash, "3 [SI::m]")),
	}})
	if err != nil {
		t.Fatalf("EvaluateCalc(misshapen): %v", err)
	}
	if !strings.Contains(calc.Error, ErrTensorShapeMismatch.Error()) {
		t.Errorf("EvaluateCalc(misshapen) error = %q, want one naming %v", calc.Error, ErrTensorShapeMismatch)
	}
}

// Each arm is advertised under its own capability. A service withholding one
// names the value as unsupported — what a client built before the arm existed
// was always sent — nested values included, and refuses one sent to it rather
// than reading it as another value.
func TestSetAndTensorCapabilities(t *testing.T) {
	ctx := context.Background()
	for _, want := range []string{CapabilitySetValues, CapabilityTensorValues} {
		found := false
		for _, c := range Capabilities() {
			found = found || c == want
		}
		if !found {
			t.Errorf("capabilities %v do not include %q", Capabilities(), want)
		}
	}

	noSets := mustNewServiceWithout(t, CapabilitySetValues)
	modelHash := mustSetTensorModel(t, noSets)
	for expr, want := range map[string]string{
		"W::s.elements":     "unsupported: set Set{1, 2, 3}",
		"W::e.elements":     "unsupported: set Set{}",
		"W::mixed.elements": `unsupported: set Set{true, 1.5, 2, "a", "b", 3 [m]}`,
	} {
		got := mustEvaluate(t, noSets, modelHash, expr)
		if got.GetSet() != nil || got.GetSequence() != nil {
			t.Errorf("%s crossed as %T without %s: %v", expr, got.GetKind(), CapabilitySetValues, got)
		}
		if got.GetNull() != want {
			t.Errorf("%s without %s = %v, want null %q", expr, CapabilitySetValues, got, want)
		}
	}
	// Tensors still cross without set_values, and a set's elements are filtered
	// like any values when the arm itself crosses.
	if got := mustEvaluate(t, noSets, modelHash, "W::cube"); got.GetTensorQuantity() == nil {
		t.Errorf("W::cube without %s = %v, want a tensor", CapabilitySetValues, got)
	}
	noComplex := mustNewServiceWithout(t, CapabilityComplexValues)
	pv := setOf(&pb.Value{Kind: &pb.Value_Complex{Complex: ComplexToProto(complex(0, 1))}})
	noComplex.filterValueCapabilities(pv)
	if pv.GetSet() == nil || !strings.Contains(pv.GetSet().GetElements()[0].GetNull(), "complex number") {
		t.Errorf("set of a complex without complex_values = %v, want the element withheld", pv)
	}

	noTensors := mustNewServiceWithout(t, CapabilityTensorValues)
	modelHash = mustSetTensorModel(t, noTensors)
	for expr, want := range map[string]string{
		"W::cube":  "unsupported: tensor quantity Tensor(2, 2, 2)[1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0] [Pa]",
		"W::plane": "unsupported: tensor quantity Tensor(2, 2)[1, 2, 3, 4] [m]",
	} {
		got := mustEvaluate(t, noTensors, modelHash, expr)
		if got.GetTensorQuantity() != nil || got.GetVectorQuantity() != nil || got.GetArray() != nil {
			t.Errorf("%s crossed as %T without %s: %v", expr, got.GetKind(), CapabilityTensorValues, got)
		}
		if got.GetNull() != want {
			t.Errorf("%s without %s = %v, want null %q", expr, CapabilityTensorValues, got, want)
		}
	}
	tensors := mustEvaluate(t, noTensors, modelHash, "W::tensors")
	if elems := tensors.GetSequence().GetElements(); len(elems) != 2 || !strings.HasPrefix(elems[0].GetNull(), "unsupported: tensor quantity") {
		t.Errorf("W::tensors without %s = %v, want the elements withheld", CapabilityTensorValues, tensors)
	}
	if got := mustEvaluate(t, noTensors, modelHash, "W::s.elements"); got.GetSet() == nil {
		t.Errorf("W::s.elements without %s = %v, want a set", CapabilityTensorValues, got)
	}

	// Sent to a service without the capability, each is refused, nested included.
	metre := mustEvaluateQuantity(t, noTensors, modelHash, "3 [SI::m]")
	line := tensorQuantityValue([]int64{1}, metre)
	sequence := func(elements ...*pb.Value) *pb.Value {
		return &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: elements}}}
	}
	for name, input := range map[string]*pb.Value{"tensor": line, "nested": sequence(line), "in a set": setOf(line)} {
		_, err := noTensors.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "W::rank", Arguments: []*pb.Value{input}})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityTensorValues) {
			t.Errorf("EvaluateCalc with %s argument without %s: err = %v, want UNIMPLEMENTED naming the capability", name, CapabilityTensorValues, err)
		}
	}
	modelHash = mustSetTensorModel(t, noSets)
	set := setOf(intValue(1))
	for name, input := range map[string]*pb.Value{"set": set, "nested": sequence(set), "in an array": arrayValue([]int64{1}, set)} {
		_, err := noSets.EvaluateCalc(ctx, &pb.EvaluateCalcRequest{ModelHash: modelHash, SymbolId: "W::sizeOf", Arguments: []*pb.Value{input}})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilitySetValues) {
			t.Errorf("EvaluateCalc with %s argument without %s: err = %v, want UNIMPLEMENTED naming the capability", name, CapabilitySetValues, err)
		}
	}
}

func TestValueCarriesSetAndTensor(t *testing.T) {
	one := intValue(1)
	set := setOf(one)
	tensor := tensorQuantityValue([]int64{1}, &pb.Quantity{Unit: "m"})
	sequence := func(elements ...*pb.Value) *pb.Value {
		return &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: elements}}}
	}
	for _, tc := range []struct {
		name       string
		value      *pb.Value
		set, tensr bool
	}{
		{"nil", nil, false, false},
		{"int", one, false, false},
		{"set", set, true, false},
		{"tensor", tensor, false, true},
		{"sequence of ints", sequence(one, one), false, false},
		{"sequence with a set", sequence(one, sequence(set)), true, false},
		{"array of tensors", arrayValue([]int64{1}, tensor), false, true},
		{"set of tensors", setOf(tensor), true, true},
		{"vector quantity", vectorQuantityValue(&pb.Quantity{Unit: "m"}), false, false},
	} {
		if got := ValueCarriesSet(tc.value); got != tc.set {
			t.Errorf("ValueCarriesSet(%s) = %v, want %v", tc.name, got, tc.set)
		}
		if got := ValueCarriesTensor(tc.value); got != tc.tensr {
			t.Errorf("ValueCarriesTensor(%s) = %v, want %v", tc.name, got, tc.tensr)
		}
	}
}
