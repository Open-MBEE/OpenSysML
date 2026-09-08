package runtime

import (
	"fmt"
	"testing"
	"unsafe"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

func TestValueConstWrapping(t *testing.T) {
	// Test that runtime.Value correctly wraps semantics.Value
	semVal := semantics.Value{Kind: semantics.ValInt, Int: 42}
	v := Value{Kind: ValConst, Const: semVal}

	if v.Kind != ValConst {
		t.Errorf("expected ValConst, got %v", v.Kind)
	}
	if v.Const.Int != 42 {
		t.Errorf("expected Int=42, got %d", v.Const.Int)
	}
}

func TestSequenceOperations(t *testing.T) {
	seq := NewSequence()
	v1 := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	v2 := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 2}}

	seq.Append(v1)
	seq.Append(v2)

	if seq.Size() != 2 {
		t.Errorf("expected size 2, got %d", seq.Size())
	}

	elem, err := seq.At(0)
	if err != nil || elem.Const.Int != 1 {
		t.Errorf("expected elem[0]=1, got %v, err=%v", elem, err)
	}
}

func TestSetOperations(t *testing.T) {
	set := NewSet()
	v1 := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	v2 := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}} // duplicate

	set.Add(v1)
	set.Add(v2) // should not increase size

	if set.Size() != 1 {
		t.Errorf("expected size 1 (dedupe), got %d", set.Size())
	}

	if !set.Contains(v1) {
		t.Error("expected set to contain v1")
	}
}

func TestSetOperationsUseExactValueEquality(t *testing.T) {
	sequenceValue := func(n int64) Value {
		seq := NewSequence()
		seq.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}})
		return NewSequenceValue(seq)
	}
	first := sequenceValue(1)
	second := sequenceValue(65537)
	if valueKeyFunc(first) != valueKeyFunc(second) {
		t.Fatal("test values must share a collection hash bucket")
	}
	set := NewSet()
	set.Add(first)
	set.Add(second)

	if set.Size() != 2 {
		t.Fatalf("expected two distinct sequence elements, got %d", set.Size())
	}
	if !set.Contains(second) {
		t.Fatal("expected set to contain the second colliding sequence")
	}
}

func TestSetTreatsEveryEmptyValueAsOneMember(t *testing.T) {
	empties := []Value{
		{Kind: ValNull},
		NewSequenceValue(NewSequence()),
		NewSetValue(NewSet()),
	}
	for _, a := range empties {
		for _, b := range empties {
			if !valueEqual(a, b) || valueKeyFunc(a) != valueKeyFunc(b) {
				t.Fatalf("%v and %v: valueEqual %v, same key %v; want both true", a, b, valueEqual(a, b), valueKeyFunc(a) == valueKeyFunc(b))
			}
		}
	}
	for i, first := range empties {
		set := NewSet()
		set.Add(first)
		for _, other := range empties {
			set.Add(other)
			if !set.Contains(other) {
				t.Fatalf("set seeded with empties[%d] does not contain %v", i, other)
			}
		}
		if set.Size() != 1 {
			t.Fatalf("set seeded with empties[%d] holds %d empty members, want 1", i, set.Size())
		}
	}

	wrap := func(v Value) Value {
		seq := NewSequence()
		seq.Append(v)
		return NewSequenceValue(seq)
	}
	set := NewSet()
	for _, e := range empties {
		set.Add(wrap(e))
	}
	if set.Size() != 1 || !set.Contains(wrap(Value{Kind: ValNull})) {
		t.Fatalf("sequences holding an empty value: %d members, want 1 found by lookup", set.Size())
	}
}

func TestSetConstructionUsesBucketedLinearWork(t *testing.T) {
	const count = 2000
	set := NewSet()
	for i := 0; i < count; i++ {
		set.Add(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: int64(i)}})
	}
	for i := 0; i < count; i++ {
		if !set.Contains(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: int64(i)}}) {
			t.Fatalf("set does not contain %d", i)
		}
	}
	if len(set.elements) != count {
		t.Fatalf("set has %d buckets for %d elements", len(set.elements), count)
	}
	maxBucket := 0
	for _, bucket := range set.elements {
		if len(bucket) > maxBucket {
			maxBucket = len(bucket)
		}
	}
	if maxBucket > 1 {
		t.Fatalf("maximum bucket length = %d, want 1 for distinct integer keys", maxBucket)
	}
}

func TestSetElementsEnumerateInCanonicalOrder(t *testing.T) {
	set := NewSet()
	for _, value := range []int64{2, 1, 2, 3} {
		set.Add(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: value}})
	}
	elements := set.Elements()
	if len(elements) != 3 {
		t.Fatalf("set has %d elements, want 3", len(elements))
	}
	for i, want := range []int64{1, 2, 3} {
		if got := elements[i].Const.Int; got != want {
			t.Errorf("element %d = %d, want %d", i, got, want)
		}
	}
}

func TestSetDeduplicatesCommensurableQuantities(t *testing.T) {
	metre := &symbols.Symbol{Name: "metre"}
	unit := Unit{
		Text: "km",
		Term: semantics.UnitTerm{
			Scale:   semantics.UnitScale(1000),
			Factors: []semantics.UnitFactor{{Unit: metre, Exponent: 1}},
		},
	}
	base := Unit{
		Text: "m",
		Term: semantics.UnitTerm{
			Scale:   semantics.UnitScale(1),
			Factors: []semantics.UnitFactor{{Unit: metre, Exponent: 1}},
		},
	}
	set := NewSet()
	set.Add(NewQuantityValue(&Quantity{
		Num:  semantics.Value{Kind: semantics.ValInt, Int: 1},
		Unit: unit,
	}))
	set.Add(NewQuantityValue(&Quantity{
		Num:  semantics.Value{Kind: semantics.ValInt, Int: 1000},
		Unit: base,
	}))
	if set.Size() != 1 {
		t.Fatalf("set size = %d, want 1 for commensurable equal quantities", set.Size())
	}
}

func TestFormatValueCollections(t *testing.T) {
	inner := NewSequence()
	inner.Append(NewStringValue("nested"))
	outer := NewSequence()
	outer.Append(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}})
	outer.Append(NewSequenceValue(inner))

	set := NewSet()
	set.Add(Value{Kind: ValInstance, Instance: 3})
	set.Add(NewSequenceValue(outer))

	if got, want := FormatValue(NewSequenceValue(outer)),
		`[1, ["nested"]]`; got != want {
		t.Errorf("sequence formatting = %q, want %q", got, want)
	}
	if got, want := FormatValue(NewSetValue(set)),
		`Set{instance(3), [1, ["nested"]]}`; got != want {
		t.Errorf("set formatting = %q, want %q", got, want)
	}
}

// Every payload accessor answers only for its own kind and the zero value for
// the rest, and the struct stays small enough to pass through evaluator frames.
func TestValuePayloadAccessorsAreKindSpecific(t *testing.T) {
	if size := unsafe.Sizeof(Value{}); size > 64 {
		t.Errorf("Value is %d bytes, want at most 64", size)
	}
	seq, set, q := NewSequence(), NewSet(), &Quantity{Num: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	variant := &symbols.Symbol{Name: "v"}
	literal := &symbols.Symbol{Name: "l"}
	body := &ast.BodyExpr{}
	values := []Value{
		{}, {Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 7}},
		{Kind: ValNull}, {Kind: ValInstance, Instance: 9},
		NewStringValue("s"), NewSequenceValue(seq), NewSetValue(set), NewExprValue(body, nil),
		NewQuantityValue(q), NewVariantValue(variant, 4), NewEnumLiteral(literal),
	}
	// A payload whose Kind was rewritten afterwards answers only to the new kind.
	for _, v := range values[4:] {
		for _, kind := range []ValueKind{ValConst, ValNull, ValInstance, ValString, ValSequence, ValSet, ValExpr, ValQuantity, ValVariant, ValEnumLiteral} {
			if kind != v.Kind {
				relabeled := v
				relabeled.Kind = kind
				values = append(values, relabeled)
			}
		}
	}
	for _, v := range values {
		if got := v.Str(); got != "" && v.Kind != ValString {
			t.Errorf("%s.Str() = %q", v.Kind, got)
		}
		if got := v.Sequence(); got != nil && v.Kind != ValSequence {
			t.Errorf("%s.Sequence() = %v", v.Kind, got)
		}
		if got := v.Set(); got != nil && v.Kind != ValSet {
			t.Errorf("%s.Set() = %v", v.Kind, got)
		}
		if got := v.Expr(); got != nil && v.Kind != ValExpr {
			t.Errorf("%s.Expr() = %v", v.Kind, got)
		}
		if got := v.Quantity(); got != nil && v.Kind != ValQuantity {
			t.Errorf("%s.Quantity() = %v", v.Kind, got)
		}
		if got := v.Variant(); got != nil && v.Kind != ValVariant {
			t.Errorf("%s.Variant() = %v", v.Kind, got)
		}
		if got := v.Literal(); got != nil && v.Kind != ValEnumLiteral {
			t.Errorf("%s.Literal() = %v", v.Kind, got)
		}
	}
	if got := NewStringValue("s").Str(); got != "s" {
		t.Errorf("Str() = %q, want \"s\"", got)
	}
	if NewSequenceValue(seq).Sequence() != seq || NewSetValue(set).Set() != set ||
		NewExprValue(body, nil).Expr() != body || NewQuantityValue(q).Quantity() != q ||
		NewVariantValue(variant, 4).Variant() != variant || NewEnumLiteral(literal).Literal() != literal {
		t.Error("an accessor did not return the payload its constructor was given")
	}
	if id, ok := NewVariantValue(variant, 4).Object(); !ok || id != 4 {
		t.Errorf("variant Object() = %d, %v, want 4, true", id, ok)
	}
	if _, ok := NewVariantValue(variant, 0).Object(); ok {
		t.Error("a variant that materialized no object reports one")
	}
}

// A Real reads as the shortest decimal that reads back as the same value: a
// magnitude two decimals cannot show reads as itself rather than as zero, a
// value carried at full precision is not rounded away, and a whole value keeps
// its ".0" so it is not mistaken for an Integer.
func TestFormatConstRealRoundTrips(t *testing.T) {
	cases := []struct {
		real float64
		want string
	}{
		{0.0001, "0.0001"},
		{1e-7, "1e-07"},
		{1.0 / 3.0, "0.3333333333333333"},
		{123456789.987654, "123456789.987654"},
		{1e21, "1e+21"},
		{1e20, "100000000000000000000.0"},
		{2, "2.0"},
		{0, "0.0"},
		{-15.200531548598184, "-15.200531548598184"},
	}
	for _, tc := range cases {
		got := semantics.FormatConst(semantics.Value{Kind: semantics.ValReal, Real: tc.real})
		if got != tc.want {
			t.Errorf("semantics.FormatConst(%v) = %q, want %q", tc.real, got, tc.want)
		}
		// A rendered Real reads back as the value it was rendered from.
		var back float64
		if _, err := fmt.Sscanf(got, "%g", &back); err != nil || back != tc.real {
			t.Errorf("semantics.FormatConst(%v) = %q, which reads back as %v (%v)", tc.real, got, back, err)
		}
	}
}
