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

func sequenceValue(elements ...*pb.Value) *pb.Value {
	return &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: elements}}}
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
	if got := runtime.FormatValue(back); got != "Set{Set{}, Set{1, 2, 3}}" {
		t.Errorf("set of sets read back as %s", got)
	}
	if got := runtime.FormatValue(displayValue(ValueToProto(back, idx))); got != "Set{Set{}, Set{1, 2, 3}}" {
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
		{"null and the empty sequence", setOf(&pb.Value{Kind: &pb.Value_Null{}}, sequenceValue()), ErrSetElementRepeated},
		{"empty set and the empty sequence", setOf(setOf(), sequenceValue()), ErrSetElementRepeated},
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

	// A set is not the sequence of its members: the two are distinct members of a set.
	val, err := ProtoToValueIn(setOf(setOf(intValue(1), intValue(2)), sequenceValue(intValue(1), intValue(2)), sequenceValue(intValue(2), intValue(1))), idx, sem)
	if err != nil || val.Kind != runtime.ValSet || val.Set().Size() != 3 {
		t.Fatalf("a set holding a set and the two sequences of its members = %s, %v, want three members", runtime.FormatValue(val), err)
	}
}

// A set holding a member with no wire form is withheld whole. Two frames read
// from two objects are two members that render alike; sent as two nulls they
// would read back as one repeated, so the null names the set instead. A
// sequence of the same still crosses member by member, its places kept.
func TestSetHoldingAMemberWithNoWireFormIsWithheldWhole(t *testing.T) {
	frames := []runtime.Value{
		runtime.NewCoordinateFrameValue(&runtime.CoordinateFrame{Object: 1, Text: "spatialCF"}),
		runtime.NewCoordinateFrameValue(&runtime.CoordinateFrame{Object: 2, Text: "spatialCF"}),
	}
	set := runtime.NewSet()
	for _, frame := range frames {
		set.Add(frame)
	}
	if set.Size() != 2 {
		t.Fatalf("two frames read from two objects make a set of %d", set.Size())
	}
	const want = "unsupported: set Set{spatialCF [], spatialCF []} holding coordinate frame spatialCF []"
	if pv := ValueToProto(runtime.NewSetValue(set), nil); pv.GetNull() != want {
		t.Errorf("set of two frames crossed as %v, want null %q", pv, want)
	}
	seq := runtime.NewSequence()
	for _, frame := range frames {
		seq.Append(frame)
	}
	seq.Append(runtime.NewSetValue(set))
	pv := ValueToProto(runtime.NewSequenceValue(seq), nil)
	elems := pv.GetSequence().GetElements()
	if len(elems) != 3 || elems[0].GetNull() != "unsupported: coordinate frame spatialCF []" || elems[2].GetNull() != want {
		t.Errorf("sequence of two frames and their set crossed as %v", pv)
	}

	// One member with a wire form beside one without withholds the set too,
	// wherever the member lies.
	mixed := runtime.NewSet()
	mixed.Add(runtime.NewStringValue("a"))
	mixed.Add(frames[0])
	if pv := ValueToProto(runtime.NewSetValue(mixed), nil); pv.GetNull() != `unsupported: set Set{"a", spatialCF []} holding coordinate frame spatialCF []` {
		t.Errorf("set of a string and a frame crossed as %v", pv)
	}
	outer := runtime.NewSet()
	outer.Add(runtime.NewSetValue(mixed))
	outer.Add(runtime.NewStringValue("b"))
	if pv := ValueToProto(runtime.NewSetValue(outer), nil); !strings.HasPrefix(pv.GetNull(), `unsupported: set Set{"b", Set{"a", spatialCF []}} holding set `) {
		t.Errorf("set nesting the set crossed as %v", pv)
	}
}

// functionSetModel reads one calc against two objects, so two function values
// render alike, and lists them in a set both ways round.
const functionSetModel = `
package G {
  private import ScalarValues::*;
  private import Collections::*;
  calc def Unary { in v : Real; return : Real; }
  part def Holder { attribute k : Real; calc scale :> Unary { in :>> v; return : Real = v * k; } }
  part a : Holder { :>> k = 2.0; }
  part b : Holder { :>> k = 3.0; }
  attribute ab : Set { :>> elements = (a.scale, b.scale); }
  attribute ba : Set { :>> elements = (b.scale, a.scale); }
}
`

// Two functions that render alike — one calc read against two objects — are
// two members, sent in one order however the set was written.
func TestSetOfAlikeFunctionsCrossesInOneOrder(t *testing.T) {
	srv := mustNewService(t, 4)
	modelHash := mustParse(t, srv, functionSetModel)
	if eq := mustEvaluate(t, srv, modelHash, "G::ab == G::ba"); !eq.GetBoolValue() {
		t.Fatalf("G::ab == G::ba = %v, want true", eq)
	}
	if ab := mustEvaluate(t, srv, modelHash, "G::ab.elements").GetSet(); ab == nil || len(ab.GetElements()) != 2 {
		t.Fatalf("G::ab.elements = %v, want a set of two functions", ab)
	}
	// Both sets flow into one sequence, so their members' objects are numbered
	// within one response and the two enumerations can be compared.
	both := mustEvaluate(t, srv, modelHash, "(G::ab.elements, G::ba.elements)").GetSequence().GetElements()
	if len(both) != 4 {
		t.Fatalf("(G::ab.elements, G::ba.elements) = %v, want four functions", both)
	}
	for i := range 2 {
		x, y := both[i].GetFunction(), both[i+2].GetFunction()
		if x == nil || y == nil || x.GetCalcId() != "G::Holder::scale" || x.GetCalcId() != y.GetCalcId() || x.GetSelfId() == 0 || x.GetSelfId() != y.GetSelfId() {
			t.Errorf("member %d: %v in ab, %v in ba, want the function G::Holder::scale over one object in both", i, x, y)
		}
	}
	if both[0].GetFunction().GetSelfId() == both[1].GetFunction().GetSelfId() {
		t.Errorf("G::ab.elements = %v, want two functions over two objects", both[:2])
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
	// Tensors still cross without set_values, and a set holding a member the
	// service withholds is withheld whole, not sent with a null in its place.
	if got := mustEvaluate(t, noSets, modelHash, "W::cube"); got.GetTensorQuantity() == nil {
		t.Errorf("W::cube without %s = %v, want a tensor", CapabilitySetValues, got)
	}
	noComplex := mustNewServiceWithout(t, CapabilityComplexValues)
	pv := setOf(intValue(1), &pb.Value{Kind: &pb.Value_Complex{Complex: ComplexToProto(complex(0, 1))}})
	noComplex.filterValueCapabilities(pv)
	if want := "unsupported: set Set{1, 0.0 + 1.0i} holding complex number 0.0 + 1.0i"; pv.GetNull() != want {
		t.Errorf("set of a complex without complex_values = %v, want null %q", pv, want)
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
