package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// tensorContext declares a 2×2 stress reference and the values the tests read.
func tensorContext(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	ctx, idx := libraryModelContext(t, `package test {
		private import ISQ::*;
		private import SI::*;
		private import MeasurementReferences::*;
		private import Quantities::*;
		private import TensorCalculations::*;
		attribute kPa : PressureUnit { :>> unitConversion : ConversionByPrefix { :>> prefix = kilo; :>> referenceUnit = Pa; } }
		attribute stressRef : TensorMeasurementReference {
			:>> dimensions = (2, 2);
			:>> mRefs = (Pa, Pa, Pa, Pa);
		}
		attribute mixedRef : TensorMeasurementReference {
			:>> dimensions = (2, 2);
			:>> mRefs = (Pa, kPa, Pa, Pa);
		}
		attribute stress = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef);
	}`)
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("test package not indexed")
	}
	return ctx, pkg.Scope
}

func tensorEval(t *testing.T, ctx *Context, scope *symbols.Scope, src string) Value {
	t.Helper()
	val, err := evalIn(t, ctx, scope, src)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return val
}

// TestTensorQuantityFormats: a tensor renders as its shape, its row-major
// components and one shared unit, or each component with its own unit.
func TestTensorQuantityFormats(t *testing.T) {
	ctx, scope := tensorContext(t)
	for src, want := range map[string]string{
		"TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef)": "Tensor(2, 2)[1.0, 2.0, 3.0, 4.0] [Pa]",
		"TensorCalculations::'['((1, 2, 3, 4), mixedRef)":          "Tensor(2, 2)[1 [Pa], 2 [kPa], 3 [Pa], 4 [Pa]]",
		"scalarQuantityTensorMult(2 [m], stress)":                  "Tensor(2, 2)[2.0, 4.0, 6.0, 8.0] [Pa*m]",
	} {
		val := tensorEval(t, ctx, scope, src)
		if val.Kind != ValTensorQuantity {
			t.Fatalf("%s = %s, want a tensor quantity", src, FormatValue(val))
		}
		if got := FormatValue(val); got != want {
			t.Errorf("FormatValue(%s) = %q, want %q", src, got, want)
		}
		if got := FormatTraceValue(val); got != want {
			t.Errorf("FormatTraceValue(%s) = %q, want %q", src, got, want)
		}
		if got := describeOperand(val); got != "a tensor quantity" {
			t.Errorf("describeOperand(%s) = %q", src, got)
		}
		if got := describeValue(val); got != "tensor quantity" {
			t.Errorf("describeValue(%s) = %q", src, got)
		}
	}
	if got := FormatTraceValue(Value{Kind: ValTensorQuantity}); got != "tensor quantity" {
		t.Errorf("a tensor value without a payload traces as %q", got)
	}
}

// TestTensorQuantityEqualityAndHashing: tensors are equal by shape and component
// quantities, however each component is spelt; one of another shape, or a vector
// of the same components, is not, and a set holds equal tensors once.
func TestTensorQuantityEqualityAndHashing(t *testing.T) {
	ctx, scope := tensorContext(t)
	stress := tensorEval(t, ctx, scope, "stress")
	same := tensorEval(t, ctx, scope, "TensorCalculations::'['((1, 2, 3, 4), stressRef)")
	converted := tensorEval(t, ctx, scope, "TensorCalculations::'['((1.0, 0.002, 3.0, 4.0), mixedRef)")
	other := tensorEval(t, ctx, scope, "TensorCalculations::'['((1.0, 2.0, 3.0, 5.0), stressRef)")
	vector := tensorEval(t, ctx, scope, "VectorFunctions::VectorOf((1.0, 2.0, 3.0, 4.0)) [Pa]")
	tall := NewTensorQuantityValue([]int64{4, 1}, stress.TensorQuantity().Num, stress.TensorQuantity().Units)

	for _, tc := range []struct {
		name string
		a, b Value
		want bool
	}{
		{"same magnitudes as integers", stress, same, true},
		{"a component in a commensurable unit", stress, converted, true},
		{"a differing component", stress, other, false},
		{"a vector of the components", stress, vector, false},
		{"another shape", stress, tall, false},
	} {
		if got := valueEqual(tc.a, tc.b); got != tc.want {
			t.Errorf("%s: valueEqual = %v, want %v", tc.name, got, tc.want)
		}
		if tc.want && valueKeyFunc(tc.a) != valueKeyFunc(tc.b) {
			t.Errorf("%s: equal tensors have different keys", tc.name)
		}
	}
	set := NewSet()
	for _, v := range []Value{stress, same, converted, other, tall, vector} {
		set.Add(v)
	}
	if set.Size() != 4 {
		t.Fatalf("set holds %d elements, want 4: %s", set.Size(), FormatTraceValue(NewSetValue(set)))
	}
}

// TestTensorQuantityFeatures: a tensor answers its Array and TensorQuantityValue
// members from its data, and its mRef is the reference object it was built over.
func TestTensorQuantityFeatures(t *testing.T) {
	ctx, scope := tensorContext(t)
	for src, want := range map[string]string{
		"stress.dimensions":                 "[2, 2]",
		"stress.rank":                       "2",
		"stress.order":                      "2",
		"stress.flattenedSize":              "4",
		"stress.elements":                   "[1.0, 2.0, 3.0, 4.0]",
		"stress.num":                        "[1.0, 2.0, 3.0, 4.0]",
		"stress.isBound":                    "false",
		"stress.mRef":                       "Array(2, 2)[Pa, Pa, Pa, Pa]",
		"stress.mRef.mRefs":                 "[Pa, Pa, Pa, Pa]",
		"stress#(2, 1)":                     "3.0 [Pa]",
		"(2 * stress).mRef":                 "Array(2, 2)[Pa, Pa, Pa, Pa]",
		"(stress + stress).mRef.dimensions": "[2, 2]",
		"(stress * (2 [m])).mRef":           "Array(2, 2)[Pa*m, Pa*m, Pa*m, Pa*m]",
	} {
		if got := FormatValue(tensorEval(t, ctx, scope, src)); got != want {
			t.Errorf("%s = %s, want %s", src, got, want)
		}
	}
	for _, src := range []string{"stress.contravariantOrder", "stress.covariantOrder", "(2 * stress).isBound"} {
		_, err := evalIn(t, ctx, scope, src)
		if !errors.Is(err, ErrUnevaluableLibraryFunction) {
			t.Errorf("%s: %v, want %v", src, err, ErrUnevaluableLibraryFunction)
		}
	}
}

// TestTensorQuantityConformsToItsDeclaredTypes: a tensor is a TensorQuantityValue
// and an Array; it is no vector or scalar quantity, so a scalar quantity type
// refuses it whatever its components measure.
func TestTensorQuantityConformsToItsDeclaredTypes(t *testing.T) {
	ctx, scope := tensorContext(t)
	stress := tensorEval(t, ctx, scope, "stress")
	for typeFQN, want := range map[string]bool{
		"Quantities::TensorQuantityValue": true,
		"Collections::Array":              true,
		"ISQ::PressureValue":              false,
		"ISQ::LengthValue":                false,
		"Quantities::VectorQuantityValue": false,
		"Quantities::ScalarQuantityValue": false,
		"ScalarValues::Real":              false,
	} {
		declared := lookupOne(t, ctx.resolver.Index(), typeFQN)
		ok, refusal, err := ctx.valueConforms(scope, &stress, declared, admitWritten)
		if err != nil {
			t.Fatalf("conformance of a tensor to %s: %v", typeFQN, err)
		}
		if ok != want {
			t.Errorf("a tensor conforms to %s = %v (%q), want %v", typeFQN, ok, refusal, want)
		}
		if typeFQN == "ISQ::PressureValue" && !strings.Contains(refusal, "holds one scalar") {
			t.Errorf("refusal of a tensor for %s = %q, want it to say the type holds one scalar", typeFQN, refusal)
		}
	}
}

// TestTensorQuantityMRefOutlivesItsReferenceOnly: a tensor built by hand names no
// reference object, so its mRef is the Array of its components' references.
func TestTensorQuantityMRefOutlivesItsReferenceOnly(t *testing.T) {
	ctx, scope := tensorContext(t)
	metre := tensorEval(t, ctx, scope, "m").MeasurementRef().Unit
	second := tensorEval(t, ctx, scope, "s").MeasurementRef().Unit
	num := []semantics.Value{{Kind: semantics.ValReal, Real: 1}, {Kind: semantics.ValReal, Real: 2}, {Kind: semantics.ValReal, Real: 3}, {Kind: semantics.ValReal, Real: 4}}
	tq := NewTensorQuantityValue([]int64{2, 2}, num, []Unit{metre, second, metre, second})
	got, ok, err := ctx.structuredFeature(tq, "mRef")
	if !ok || err != nil {
		t.Fatalf("mRef of %s: %v, %v", FormatValue(tq), ok, err)
	}
	if want := "Array(2, 2)[m, s, m, s]"; FormatValue(got) != want {
		t.Errorf("mRef = %s, want %s", FormatValue(got), want)
	}
	if got := FormatValue(tq); !strings.HasPrefix(got, "Tensor(2, 2)[1.0 [m], 2.0 [s]") {
		t.Errorf("a tensor of mixed units formats as %s", got)
	}
	ctx.maxElements = ctx.elements + 3
	if _, _, err := ctx.structuredFeature(tq, "mRef"); !errors.Is(err, ErrElementLimitExceeded) {
		t.Errorf("mRef of four references over a budget of three = %v, want ErrElementLimitExceeded", err)
	}
}

// TestAbandonedTensorReferenceIsForgottenByItsHolders: a tensor written over a
// reference object keeps it, and is unmaterialized with it when its creation is undone.
func TestAbandonedTensorReferenceIsForgottenByItsHolders(t *testing.T) {
	ctx, idx := libraryModelContext(t, `package test {
		private import ISQ::*;
		private import SI::*;
		private import MeasurementReferences::*;
		private import Quantities::*;
		attribute def Labeled :> TensorMeasurementReference { attribute label : ScalarValues::String; }
		attribute stressRef : Labeled { :>> dimensions = (2, 2); :>> mRefs = (Pa, Pa, Pa, Pa); :>> label = "stress"; }
		part def Holder { attribute stress : TensorQuantityValue; }
		part holder : Holder;
	}`)
	pkg, _ := idx.DocumentRoot("<test>").LookupLocal("test")
	holder, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "holder"))
	if err != nil {
		t.Fatal(err)
	}
	mark := len(ctx.created)
	val := tensorEval(t, ctx, pkg.Scope, "TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef)")
	if ref := val.TensorQuantity().MRef; ctx.instances[ref] == nil {
		t.Fatalf("the tensor is built over object %d, which the session does not hold", ref)
	}
	if err := holder.SetFeatureValue(ctx, "stress", val); err != nil {
		t.Fatalf("write stress: %v", err)
	}
	if label := tensorEval(t, ctx, pkg.Scope, "holder.stress.mRef.label"); FormatValue(label) != `"stress"` {
		t.Fatalf("holder.stress.mRef.label = %s, want \"stress\"", FormatValue(label))
	}
	stress := holder.FeatureValues["stress"]
	if stress == nil || !stress.Materialized || stress.Value.Kind != ValTensorQuantity {
		t.Fatalf("stress = %+v, want a materialized tensor quantity", stress)
	}

	ctx.abandonInstancesSince(mark)
	if stress.Materialized {
		t.Error("stress still holds a tensor built over an abandoned reference")
	}
}

// TestWriteOfEqualTensorComponentsOverAnotherReferenceRecomputesDerivedValues: a
// tensor of the same components over a reference of another boundness, or another
// reference object, is a change to what read its isBound or mRef.
func TestWriteOfEqualTensorComponentsOverAnotherReferenceRecomputesDerivedValues(t *testing.T) {
	ctx, idx := libraryModelContext(t, `package test {
		private import ISQ::*;
		private import SI::*;
		private import MeasurementReferences::*;
		private import Quantities::*;
		attribute def Labeled :> TensorMeasurementReference { attribute label : ScalarValues::String; }
		attribute looseRef : Labeled { :>> dimensions = (2, 2); :>> mRefs = (Pa, Pa, Pa, Pa); :>> label = "loose"; }
		attribute boundRef : Labeled { :>> dimensions = (2, 2); :>> mRefs = (Pa, Pa, Pa, Pa); :>> isBound = true; :>> label = "bound"; }
		attribute otherRef : Labeled { :>> dimensions = (2, 2); :>> mRefs = (Pa, Pa, Pa, Pa); :>> isBound = true; :>> label = "other"; }
		part def Holder {
			attribute stress : TensorQuantityValue;
			attribute bound = stress.isBound;
			attribute label = stress.mRef.label;
		}
		part holder : Holder;
	}`)
	pkg, _ := idx.DocumentRoot("<test>").LookupLocal("test")
	holder, err := ctx.Instantiate(resolveSymbol(t, pkg.Scope, "holder"))
	if err != nil {
		t.Fatal(err)
	}
	write := func(ref string) {
		t.Helper()
		val := tensorEval(t, ctx, pkg.Scope, "TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), "+ref+")")
		if err := holder.SetFeatureValue(ctx, "stress", val); err != nil {
			t.Fatalf("write stress over %s: %v", ref, err)
		}
	}
	expect := func(ref, bound, label string) {
		t.Helper()
		if got := readFormatted(t, ctx, holder, "bound"); got != bound {
			t.Errorf("bound over %s = %s, want %s", ref, got, bound)
		}
		if got := readFormatted(t, ctx, holder, "label"); got != label {
			t.Errorf("label over %s = %s, want %s", ref, got, label)
		}
	}
	write("looseRef")
	expect("looseRef", "false", `"loose"`)
	write("boundRef")
	if holder.FeatureValues["bound"].Materialized || holder.FeatureValues["label"].Materialized {
		t.Fatal("bound or label is still materialized after stress was restated over boundRef")
	}
	expect("boundRef", "true", `"bound"`)
	write("otherRef")
	if holder.FeatureValues["label"].Materialized {
		t.Fatal("label is still materialized after stress was restated over otherRef")
	}
	expect("otherRef", "true", `"other"`)
	write("otherRef")
	if !holder.FeatureValues["bound"].Materialized || !holder.FeatureValues["label"].Materialized {
		t.Fatal("writing the same tensor over the same reference again unmaterialized its readers")
	}
}

// TestTensorQuantityRankThree: a tensor of rank three constructs, indexes with
// one index per dimension in row-major order, keeps its shape through the
// arithmetic, renders canonically, and is equal to a same-shape tensor of the
// same components only.
func TestTensorQuantityRankThree(t *testing.T) {
	ctx, idx := libraryModelContext(t, `package test {
		private import ISQ::*;
		private import SI::*;
		private import MeasurementReferences::*;
		private import Quantities::*;
		attribute cubeRef : TensorMeasurementReference {
			:>> dimensions = (2, 2, 2);
			:>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa);
		}
		attribute cube = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef);
	}`)
	pkg, _ := idx.DocumentRoot("<test>").LookupLocal("test")
	scope := pkg.Scope
	cube := tensorEval(t, ctx, scope, "cube")
	if cube.Kind != ValTensorQuantity || cube.TensorQuantity().Rank() != 3 || cube.TensorQuantity().FlattenedSize() != 8 {
		t.Fatalf("cube = %s, want a rank-3 tensor of 8 components", FormatValue(cube))
	}
	if got, want := FormatTraceValue(cube), "Tensor(2, 2, 2)[1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0] [Pa]"; got != want {
		t.Errorf("trace = %q, want %q", got, want)
	}
	for src, want := range map[string]float64{
		"cube#(1, 1, 1)": 1, "cube#(1, 1, 2)": 2, "cube#(1, 2, 1)": 3, "cube#(2, 1, 1)": 5, "cube#(2, 2, 2)": 8,
		"(cube + cube)#(2, 1, 2)": 12, "(2 * cube)#(1, 2, 2)": 8, "(cube - cube)#(2, 2, 1)": 0,
	} {
		val := tensorEval(t, ctx, scope, src)
		if val.Kind != ValQuantity || val.Quantity().Unit.Text != "Pa" || numberOf(val.Quantity().Num) != want {
			t.Errorf("%s = %s, want %v [Pa]", src, FormatValue(val), want)
		}
	}
	for _, src := range []string{"cube + cube", "2 * cube", "cube - cube"} {
		val := tensorEval(t, ctx, scope, src)
		if val.Kind != ValTensorQuantity || !strings.HasPrefix(FormatValue(val), "Tensor(2, 2, 2)[") {
			t.Errorf("%s = %s, want the shape kept", src, FormatValue(val))
		}
	}
	tq := cube.TensorQuantity()
	if !valueEqual(cube, NewTensorQuantityValue([]int64{2, 2, 2}, tq.Num, tq.Units)) {
		t.Error("a same-shape tensor of the same components is not equal")
	}
	if valueEqual(cube, NewTensorQuantityValue([]int64{2, 4}, tq.Num, tq.Units)) {
		t.Error("a reshaped tensor of the same components is equal")
	}
	for src, want := range map[string]error{
		"cube#(1, 2)": ErrMultiplicityViolation, "cube#(1, 1, 1, 1)": ErrMultiplicityViolation,
		"cube#(0, 1, 1)": ErrIndexOutOfRange, "cube#(1, 1, 3)": ErrIndexOutOfRange,
	} {
		if _, err := evalIn(t, ctx, scope, src); !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", src, err, want)
		}
	}
}
