package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// setModel declares one of each library Collection, valued from the same
// repeating numbers, and features a Set's elements flow into.
const setModel = `package test {
	private import ScalarValues::*;
	private import Collections::*;
	private import CollectionFunctions::*;
	private import ControlFunctions::*;
	private import SequenceFunctions::*;

	attribute s : Set { :>> elements = (3, 1, 2, 2, 3); }
	attribute t : Set { :>> elements = (2, 3, 1); }
	attribute e : Set { :>> elements = (); }
	attribute u : UniqueCollection { :>> elements = (3, 1, 2, 2); }
	attribute kv1 : KeyValuePair { :>> key = 1; :>> val = "a"; }
	attribute kv2 : KeyValuePair { :>> key = 2; :>> val = "b"; }
	attribute m : Map { :>> elements = (kv1, kv2, kv1); }
	attribute om : OrderedMap { :>> elements = (kv2, kv1); }
	attribute b : Bag { :>> elements = (3, 1, 2, 2); }
	attribute os : OrderedSet { :>> elements = (3, 1, 2); }
	attribute l : List { :>> elements = (3, 1, 2, 2); }
	attribute arr : Array { :>> elements = (3, 1, 2, 2); :>> dimensions = (4); }
	attribute nested : Set { :>> elements = (s, e, s); }
	attribute mixed : Set { :>> elements = (2, "a", true, 1.5, 1, "a"); }
	enum def Color { red; green; }
	package Other { enum def Color { red; } }
	attribute lits : Set { :>> elements = (Color::red, Other::Color::red, Color::green); }
	attribute stil : Set { :>> elements = (Other::Color::red, Color::green, Color::red); }

	attribute plain : Integer[*] = s.elements;
	attribute ordered : Integer[*] ordered = s.elements;
	attribute repeatable : Integer[*] nonunique = s.elements;
	attribute fromSet : List { :>> elements = s.elements; }
	attribute fromList : Set { :>> elements = l.elements; }
	attribute fromBag : Set { :>> elements = b.elements; }

	part def P { attribute ints : Integer[*]; attribute ord : Integer[*] ordered; attribute chosen : Set; }
	part p : P { :>> ints = s.elements; :>> ord = s.elements; :>> chosen { :>> elements = l.elements; } }
}`

func setModelContext(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	ctx, idx := libraryModelContext(t, setModel)
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok {
		t.Fatal("package test not found")
	}
	return ctx, pkg.Scope
}

func mustEvalIn(t *testing.T, ctx *Context, scope *symbols.Scope, src string) Value {
	t.Helper()
	val, err := evalIn(t, ctx, scope, src)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return val
}

// TestCollectionElementsHoldTheLibraryKind pins which library Collections hold
// their elements as a set: those the library declares unique and not ordered.
// Everything ordered or nonunique stays the sequence it was written as.
func TestCollectionElementsHoldTheLibraryKind(t *testing.T) {
	ctx, scope := setModelContext(t)
	sets := map[string][]int64{
		"s.elements": {1, 2, 3},
		"t.elements": {1, 2, 3},
		"e.elements": {},
		"u.elements": {1, 2, 3},
	}
	for src, want := range sets {
		val := mustEvalIn(t, ctx, scope, src)
		if val.Kind != ValSet {
			t.Errorf("%s: kind = %v, want a set", src, val.Kind)
			continue
		}
		if got := intsOf(t, sequenceOf(elementsOf(val))); !equalInts(got, want) {
			t.Errorf("%s = %v, want %v", src, got, want)
		}
	}
	sequences := map[string][]int64{
		"b.elements":   {3, 1, 2, 2},
		"os.elements":  {3, 1, 2},
		"l.elements":   {3, 1, 2, 2},
		"arr.elements": {3, 1, 2, 2},
	}
	for src, want := range sequences {
		val := mustEvalIn(t, ctx, scope, src)
		if val.Kind != ValSequence {
			t.Errorf("%s: kind = %v, want a sequence", src, val.Kind)
			continue
		}
		if got := intsOf(t, val); !equalInts(got, want) {
			t.Errorf("%s = %v, want %v", src, got, want)
		}
	}
	for src, kind := range map[string]ValueKind{"m.elements": ValSet, "om.elements": ValSequence} {
		val := mustEvalIn(t, ctx, scope, src)
		if val.Kind != kind || len(elementsOf(val)) != 2 {
			t.Errorf("%s = %s, want two pairs as a %v", src, FormatValue(val), kind)
		}
	}
	if val := mustEvalIn(t, ctx, scope, "nested.elements"); val.Kind != ValSet || val.Set().Size() != 2 {
		t.Errorf("nested.elements = %s, want a set of the two distinct sets", FormatValue(val))
	}
}

// TestSetFlowsIntoDeclaredCollections pins the boundary rule: a set read into a
// feature the runtime holds as a sequence — ordered, nonunique, a plain
// `Integer[*]`, a List's elements, an inherited or redefined feature — becomes
// its canonical sequence, and a sequence read into a Set's elements becomes the
// set of its distinct elements.
func TestSetFlowsIntoDeclaredCollections(t *testing.T) {
	ctx, scope := setModelContext(t)
	for _, src := range []string{"plain", "ordered", "repeatable", "fromSet.elements", "p.ints", "p.ord"} {
		val := mustEvalIn(t, ctx, scope, src)
		if val.Kind != ValSequence {
			t.Errorf("%s: kind = %v, want a sequence", src, val.Kind)
			continue
		}
		if got := intsOf(t, val); !equalInts(got, []int64{1, 2, 3}) {
			t.Errorf("%s = %v, want the set's canonical sequence (1, 2, 3)", src, got)
		}
	}
	for _, src := range []string{"fromList.elements", "fromBag.elements", "p.chosen.elements"} {
		val := mustEvalIn(t, ctx, scope, src)
		if val.Kind != ValSet {
			t.Errorf("%s: kind = %v, want a set", src, val.Kind)
			continue
		}
		if got := intsOf(t, sequenceOf(elementsOf(val))); !equalInts(got, []int64{1, 2, 3}) {
			t.Errorf("%s = %v, want the distinct elements {1, 2, 3}", src, got)
		}
	}
}

// TestCollectionFunctionsOverCollectionObjects pins that CollectionFunctions
// read a Collection object through its elements, whichever kind it holds.
func TestCollectionFunctionsOverCollectionObjects(t *testing.T) {
	ctx, scope := setModelContext(t)
	ints := map[string]int64{
		"size(s)": 3, "size(e)": 0, "size(b)": 4, "size(m)": 2,
		"CollectionFunctions::head(s)": 1, "CollectionFunctions::last(s)": 3,
	}
	for src, want := range ints {
		if val := mustEvalIn(t, ctx, scope, src); val.Kind != ValConst || val.Const.Int != want {
			t.Errorf("%s = %s, want %d", src, FormatValue(val), want)
		}
	}
	bools := map[string]bool{
		"isEmpty(e)": true, "isEmpty(s)": false, "notEmpty(s)": true, "notEmpty(e)": false,
		"contains(s, 2)": true, "contains(s, 5)": false, "contains(e, 2)": false,
		"containsAll(s, t)": true, "containsAll(t, s)": true, "containsAll(s, e)": true, "containsAll(s, b)": true,
		"containsAll(e, s)": false, "containsAll(s, os)": true,
		"s == t": true, "t == s": true, "s == e": false, "s != e": true, "e == e": true,
		"s == os": false, "s == b": false, "os == os": true,
		"s.elements == t.elements": true, "s.elements != e.elements": true,
	}
	for src, want := range bools {
		if val := mustEvalIn(t, ctx, scope, src); val.Kind != ValConst || val.Const.Kind != semantics.ValBool || val.Const.Bool != want {
			t.Errorf("%s = %s, want %v", src, FormatValue(val), want)
		}
	}
	if got := intsOf(t, mustEvalIn(t, ctx, scope, "CollectionFunctions::tail(s)")); !equalInts(got, []int64{2, 3}) {
		t.Errorf("tail(s) = %v, want (2, 3)", got)
	}
	if val := mustEvalIn(t, ctx, scope, "CollectionFunctions::head(e)"); val.Kind != ValNull {
		t.Errorf("head(e) = %s, want null", FormatValue(val))
	}
}

// TestSetAgainstSequenceComparesCanonically pins that a set meeting a sequence
// in `==` is its canonical sequence: equal to the elements in canonical order,
// not to the order the set happened to be written in.
func TestSetAgainstSequenceComparesCanonically(t *testing.T) {
	ctx, scope := setModelContext(t)
	for src, want := range map[string]bool{
		"s.elements == (1, 2, 3)":    true,
		"(1, 2, 3) == s.elements":    true,
		"s.elements == (3, 1, 2)":    false,
		"s.elements == (1, 2, 2, 3)": false,
		"e.elements == ()":           true,
		"s.elements->head() == 1":    true,
		"s.elements#(3) == 3":        true,
	} {
		if val := mustEvalIn(t, ctx, scope, src); val.Kind != ValConst || val.Const.Bool != want {
			t.Errorf("%s = %s, want %v", src, FormatValue(val), want)
		}
	}
	if got := intsOf(t, mustEvalIn(t, ctx, scope, "s.elements->select{in x; x > 1}")); !equalInts(got, []int64{2, 3}) {
		t.Errorf("select over a set = %v, want (2, 3)", got)
	}
}

// TestSetRendersCanonically pins the value and trace forms of a set over
// several value classes: Booleans, numbers, then strings, never map order.
func TestSetRendersCanonically(t *testing.T) {
	ctx, scope := setModelContext(t)
	val := mustEvalIn(t, ctx, scope, "mixed.elements")
	if got, want := FormatTraceValue(val), `{true, 1, 1.5, 2, "a"}`; got != want {
		t.Errorf("trace = %s, want %s", got, want)
	}
	if got, want := FormatValue(val), `Set{true, 1, 1.5, 2, "a"}`; got != want {
		t.Errorf("value = %s, want %s", got, want)
	}
	if got, want := FormatTraceValue(mustEvalIn(t, ctx, scope, "e.elements")), "{}"; got != want {
		t.Errorf("empty trace = %s, want %s", got, want)
	}
}

// TestCanonicalOrderIsTotal pins the order over one class of values that has
// no order of its own: quantities of different dimensions group by dimension,
// then by magnitude; integers beyond a double's precision stay distinct and
// ordered; and the same members added in any order enumerate alike.
func TestCanonicalOrderIsTotal(t *testing.T) {
	metre, second := &symbols.Symbol{Name: "metre"}, &symbols.Symbol{Name: "second"}
	unit := func(text string, scale float64, base *symbols.Symbol) Unit {
		return Unit{Text: text, Term: semantics.UnitTerm{Scale: semantics.UnitScale(scale), Factors: []semantics.UnitFactor{{Unit: base, Exponent: 1}}}}
	}
	quantity := func(n int64, u Unit) Value {
		return NewQuantityValue(&Quantity{Num: semantics.Value{Kind: semantics.ValInt, Int: n}, Unit: u})
	}
	km, m, s := unit("km", 1000, metre), unit("m", 1, metre), unit("s", 1, second)
	members := []Value{quantity(2, km), quantity(500, m), quantity(1, s), quantity(3, m)}
	want := []string{"3 [m]", "500 [m]", "2 [km]", "1 [s]"}
	for _, order := range [][]int{{0, 1, 2, 3}, {3, 2, 1, 0}, {2, 0, 3, 1}} {
		set := NewSet()
		for _, i := range order {
			set.Add(members[i])
		}
		var got []string
		for _, elem := range set.Elements() {
			got = append(got, FormatTraceValue(elem))
		}
		if len(got) != len(want) {
			t.Fatalf("added in %v: %v, want %v", order, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("added in %v: element %d = %s, want %s", order, i, got[i], want[i])
			}
		}
	}

	big := NewSet()
	for _, n := range []int64{1 << 53, 1<<53 + 1, 1<<53 - 1} {
		big.Add(Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}})
	}
	if got := intsOf(t, sequenceOf(big.Elements())); !equalInts(got, []int64{1<<53 - 1, 1 << 53, 1<<53 + 1}) {
		t.Errorf("large integers = %v, want them ascending", got)
	}
}

// TestSameNamedLiteralsOrderByDeclaration pins that two distinct literals whose
// enumerations share a name — rendered alike — still take one position each,
// so equal sets enumerate alike whatever order they were written in.
func TestSameNamedLiteralsOrderByDeclaration(t *testing.T) {
	ctx, scope := setModelContext(t)
	lits, stil := mustEvalIn(t, ctx, scope, "lits.elements"), mustEvalIn(t, ctx, scope, "stil.elements")
	if lits.Set().Size() != 3 || !valueEqual(lits, stil) {
		t.Fatalf("lits = %s, stil = %s, want three equal members", FormatValue(lits), FormatValue(stil))
	}
	if got, want := FormatTraceValue(lits), "{Color::green, Color::red, Color::red}"; got != want {
		t.Errorf("trace = %s, want %s", got, want)
	}
	a, b := lits.Set().Elements(), stil.Set().Elements()
	for i := range a {
		if a[i].Literal() != b[i].Literal() {
			t.Errorf("element %d differs between insertion orders: %s in %v, %s in %v", i, symbols.FQNOf(a[i].Literal()), a, symbols.FQNOf(b[i].Literal()), b)
		}
	}
	if a[1].Literal() == a[2].Literal() || symbols.FQNOf(a[1].Literal()) != "test::Color::red" || symbols.FQNOf(a[2].Literal()) != "test::Other::Color::red" {
		t.Errorf("same-named literals = %s, %s, want test::Color::red before test::Other::Color::red", symbols.FQNOf(a[1].Literal()), symbols.FQNOf(a[2].Literal()))
	}
}

// TestSetMembersEqualAcrossCollectionKinds pins that a sequence and the set it
// equals — valueEqual holds across the two kinds — share one key, so a set
// admits only one of them and finds either.
func TestSetMembersEqualAcrossCollectionKinds(t *testing.T) {
	ints := func(ns ...int64) []Value {
		vals := make([]Value, len(ns))
		for i, n := range ns {
			vals[i] = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
		}
		return vals
	}
	seq, set, other := sequenceOf(ints(1, 2)), setOf(ints(2, 1)), sequenceOf(ints(2, 1))
	if valueKeyFunc(seq) != valueKeyFunc(set) {
		t.Errorf("keys differ for %s and %s", FormatValue(seq), FormatValue(set))
	}
	for _, members := range [][]Value{{seq, set, other}, {set, seq, other}, {other, set, seq}} {
		outer := setOf(members).Set()
		if outer.Size() != 2 {
			t.Errorf("set of %v has %d members, want 2", members, outer.Size())
		}
		for _, m := range []Value{seq, set, other} {
			if !outer.Contains(m) {
				t.Errorf("set of %v lacks %s", members, FormatValue(m))
			}
		}
	}
	nested := setOf([]Value{setOf(ints(1, 2)), setOf(ints(2, 1))}).Set()
	if nested.Size() != 1 {
		t.Errorf("set of two equal sets has %d members, want 1", nested.Size())
	}
}

// TestLikeRenderedValuesOrderByContents pins that two unequal structured values
// whose trace text is the same — a unit spelt alike that reduces differently —
// still take one position each, whatever order they were added in.
func TestLikeRenderedValuesOrderByContents(t *testing.T) {
	metre := &symbols.Symbol{Name: "metre"}
	unit := func(scale float64) Unit {
		return Unit{
			Text:    "m",
			Product: semantics.NamedUnitProduct(metre, "m", false),
			Term:    semantics.UnitTerm{Scale: semantics.UnitScale(scale), Factors: []semantics.UnitFactor{{Unit: metre, Exponent: 1}}},
		}
	}
	nums := func(ns ...int64) []semantics.Value {
		out := make([]semantics.Value, len(ns))
		for i, n := range ns {
			out[i] = semantics.Value{Kind: semantics.ValInt, Int: n}
		}
		return out
	}
	m, km := unit(1), unit(1000)
	pairs := map[string][2]Value{
		"tensor": {
			NewTensorQuantityValue([]int64{2}, nums(1, 2), []Unit{m, m}),
			NewTensorQuantityValue([]int64{2}, nums(1, 2), []Unit{km, km}),
		},
		"vector": {
			NewVectorQuantityValue(nums(1, 2), []Unit{m, m}),
			NewVectorQuantityValue(nums(1, 2), []Unit{km, km}),
		},
		"mref": {NewMeasurementRefValue(m), NewMeasurementRefValue(km)},
		"array": {
			NewArrayValue([]int64{1}, []Value{NewMeasurementRefValue(m)}),
			NewArrayValue([]int64{1}, []Value{NewMeasurementRefValue(km)}),
		},
	}
	for name, pair := range pairs {
		a, b := pair[0], pair[1]
		if FormatTraceValue(a) != FormatTraceValue(b) || valueEqual(a, b) {
			t.Fatalf("%s: %s and %s should render alike and differ", name, FormatTraceValue(a), FormatTraceValue(b))
		}
		if canonicalCompare(a, b) == 0 || canonicalCompare(a, b) != -canonicalCompare(b, a) {
			t.Errorf("%s: compare(a, b) = %d, compare(b, a) = %d, want opposite and non-zero", name, canonicalCompare(a, b), canonicalCompare(b, a))
		}
		first, second := setOf([]Value{a, b}).Set(), setOf([]Value{b, a}).Set()
		if first.Size() != 2 || !first.Equal(second) {
			t.Fatalf("%s: sets differ: %s, %s", name, FormatValue(NewSetValue(first)), FormatValue(NewSetValue(second)))
		}
		x, y := first.Elements(), second.Elements()
		for i := range x {
			if !valueEqual(x[i], y[i]) {
				t.Errorf("%s: element %d differs between insertion orders", name, i)
			}
		}
	}
}
