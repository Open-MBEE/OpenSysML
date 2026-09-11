package runtime

import (
	"errors"
	"math"
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

// TestSetBoundaryUnderUniqueness: a Set's elements stay a ValSet dropping repeats,
// `ordered nonunique` sequence functions keep them, a unique sequence refuses them.
func TestSetBoundaryUnderUniqueness(t *testing.T) {
	ctx, idx := libraryModelContext(t, `package test {
		private import ScalarValues::*;
		private import Collections::*;
		private import CollectionFunctions::*;
		private import SequenceFunctions::*;
		private import ControlFunctions::*;

		attribute s : Set { :>> elements = (3, 1, 2, 2, 3); }
		attribute os : OrderedSet { :>> elements = (3, 1, 2); }
		attribute l : List { :>> elements = (3, 1, 2, 2); }
		attribute fromIncluding : Set { :>> elements = including(l.elements, 2); }
		attribute fromUnion : Set { :>> elements = union(l.elements, os.elements); }
		attribute fromCalc : Set { :>> elements = Doubled(l.elements); }
		attribute unionAsSequence : Integer[*] ordered nonunique = union(s.elements, s.elements);
		attribute fromSetAsSequence : Integer[*] ordered = including(s.elements, 4);
		attribute repeated : Integer[*] ordered = union(s.elements, s.elements);
		calc def Doubled { in xs : Integer[*] nonunique; return : Integer[*] ordered nonunique = xs->collect{in x; 2 * x}; }
	}`)
	pkg, ok := idx.DocumentRoot("<test>").LookupLocal("test")
	if !ok {
		t.Fatal("package test not found")
	}
	scope := pkg.Scope
	sets := map[string][]int64{
		"fromIncluding.elements": {1, 2, 3},
		"fromUnion.elements":     {1, 2, 3},
		"fromCalc.elements":      {2, 4, 6},
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
		"union(l.elements, os.elements)":          {3, 1, 2, 2, 3, 1, 2},
		"including(l.elements, 2)":                {3, 1, 2, 2, 2},
		"intersection(l.elements, (2, 2))":        {2, 2},
		"excluding(l.elements, 1)":                {3, 2, 2},
		"CollectionFunctions::tail(os)":           {1, 2},
		"unionAsSequence":                         {1, 2, 3, 1, 2, 3},
		"fromSetAsSequence":                       {1, 2, 3, 4},
		"including(s.elements, 2)":                {1, 2, 3, 2},
		"CollectionFunctions::tail(os) == (1, 2)": nil,
	}
	for src, want := range sequences {
		val := mustEvalIn(t, ctx, scope, src)
		if want == nil {
			if val.Kind != ValConst || !val.Const.Bool {
				t.Errorf("%s = %s, want true", src, FormatValue(val))
			}
			continue
		}
		if val.Kind != ValSequence {
			t.Errorf("%s: kind = %v, want a sequence", src, val.Kind)
			continue
		}
		if got := intsOf(t, val); !equalInts(got, want) {
			t.Errorf("%s = %v, want %v", src, got, want)
		}
	}
	_, err := evalIn(t, ctx, scope, "repeated")
	if !errors.Is(err, ErrUniquenessViolation) {
		t.Errorf("repeated = %v, want ErrUniquenessViolation: a unique sequence-held feature refuses the repeat a set would drop", err)
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

// TestMixedNumbersOrderExactly pins that an Integer is never rounded to a
// double to place it among Reals: beyond 2^53 a nearby Real takes its own
// position, and the same members added in either order enumerate alike.
func TestMixedNumbersOrderExactly(t *testing.T) {
	integer := func(n int64) Value {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
	}
	dbl := func(r float64) Value { return realConst(r) }
	for _, tc := range []struct {
		a, b Value
		want int
	}{
		{integer(1<<53 + 1), dbl(1 << 53), 1},
		{integer(1<<53 - 1), dbl(1 << 53), -1},
		{integer(1 << 53), dbl(1 << 53), 0},
		{integer(math.MaxInt64), dbl(1 << 62), 1},
		{integer(math.MaxInt64), dbl(1 << 63), -1},
		{integer(math.MinInt64), dbl(math.MinInt64), 0},
		{integer(math.MinInt64), dbl(-(1 << 64)), 1},
		{integer(2), dbl(2.5), -1},
		{integer(3), dbl(2.5), 1},
		{integer(-3), dbl(-2.5), -1},
		{integer(-2), dbl(-2.5), 1},
		{integer(0), dbl(math.Inf(1)), -1},
		{integer(0), dbl(math.Inf(-1)), 1},
	} {
		if got := canonicalCompare(tc.a, tc.b); got != tc.want {
			t.Errorf("compare(%s, %s) = %d, want %d", FormatValue(tc.a), FormatValue(tc.b), got, tc.want)
		}
		if got := canonicalCompare(tc.b, tc.a); got != -tc.want {
			t.Errorf("compare(%s, %s) = %d, want %d", FormatValue(tc.b), FormatValue(tc.a), got, -tc.want)
		}
	}

	members := []Value{integer(1<<53 + 1), dbl(1 << 53), integer(1<<53 - 1), dbl(1<<53 + 2)}
	want := "{9007199254740991, 9007199254740992.0, 9007199254740993, 9007199254740994.0}"
	var sets []*Set
	for _, order := range [][]int{{0, 1, 2, 3}, {3, 2, 1, 0}, {1, 3, 0, 2}} {
		set := NewSet()
		for _, i := range order {
			set.Add(members[i])
		}
		if set.Size() != len(members) {
			t.Fatalf("added in %v: %d members, want %d", order, set.Size(), len(members))
		}
		if got := FormatTraceValue(NewSetValue(set)); got != want {
			t.Errorf("added in %v: %s, want %s", order, got, want)
		}
		sets = append(sets, set)
	}
	for _, set := range sets[1:] {
		if !sets[0].Equal(set) {
			t.Errorf("sets of the same members differ: %s vs %s", FormatTraceValue(NewSetValue(sets[0])), FormatTraceValue(NewSetValue(set)))
		}
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

// TestSetIsNotASequenceAsAMember pins that a set and the sequence `==` reads it
// as are two members of a set — a set is never the sequence of its members —
// while two equal sets, or any two empty values, are one.
func TestSetIsNotASequenceAsAMember(t *testing.T) {
	ints := func(ns ...int64) []Value {
		vals := make([]Value, len(ns))
		for i, n := range ns {
			vals[i] = Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
		}
		return vals
	}
	seq, set, other := sequenceOf(ints(1, 2)), setOf(ints(2, 1)), sequenceOf(ints(2, 1))
	if !equalValues(seq, set) || equalValues(other, set) {
		t.Errorf("== reads %s against %s and %s in canonical order", FormatValue(set), FormatValue(seq), FormatValue(other))
	}
	if valueEqual(seq, set) || valueKeyFunc(seq) == valueKeyFunc(set) {
		t.Errorf("%s and %s are one member", FormatValue(seq), FormatValue(set))
	}
	for _, members := range [][]Value{{seq, set, other}, {set, seq, other}, {other, set, seq}} {
		outer := setOf(members).Set()
		if outer.Size() != 3 {
			t.Errorf("set of %v has %d members, want 3", members, outer.Size())
		}
		for _, m := range []Value{seq, set, other} {
			if !outer.Contains(m) {
				t.Errorf("set of %v lacks %s", members, FormatValue(m))
			}
		}
		if got, want := FormatTraceValue(NewSetValue(outer)), "{(1, 2), (2, 1), {1, 2}}"; got != want {
			t.Errorf("set of %v renders %s, want %s", members, got, want)
		}
	}
	nested := setOf([]Value{setOf(ints(1, 2)), setOf(ints(2, 1))}).Set()
	if nested.Size() != 1 {
		t.Errorf("set of two equal sets has %d members, want 1", nested.Size())
	}
	// A set holding a sequence is not the set holding the set of its elements.
	if valueEqual(setOf([]Value{seq}), setOf([]Value{set})) {
		t.Errorf("%s and %s are one value", FormatValue(setOf([]Value{seq})), FormatValue(setOf([]Value{set})))
	}
	empties := setOf([]Value{{Kind: ValNull}, sequenceOf(nil), setOf(nil)}).Set()
	if empties.Size() != 1 {
		t.Errorf("null, () and {} are %d members, want 1", empties.Size())
	}
}

// TestFunctionsRenderedAlikeOrderByIdentity pins that two distinct functions
// with one trace rendering — one calc read against two objects, or within two
// runs, or two declarations of one name — take one position each, so equal
// sets of functions enumerate alike whatever order they were written in.
func TestFunctionsRenderedAlikeOrderByIdentity(t *testing.T) {
	shape := &calcShape{Sym: &symbols.Symbol{Name: "scale"}, Name: "P::scale"}
	twin := &calcShape{Sym: &symbols.Symbol{Name: "scale", DocName: "other"}, Name: "P::scale"}
	fn := func(shape *calcShape, self *Instance, run int64) Value {
		f := &functionValue{shape: shape, self: self}
		if run != 0 {
			f.enclosing = []frame{{vars: map[string]Value{"k": integerValue(1)}, run: run}}
		}
		return Value{Kind: ValFunction, ref: f}
	}
	one, two := &Instance{ID: 1}, &Instance{ID: 2}
	pairs := map[string][2]Value{
		"two objects":      {fn(shape, one, 0), fn(shape, two, 0)},
		"two runs":         {fn(shape, one, 1), fn(shape, one, 2)},
		"two declarations": {fn(shape, one, 0), fn(twin, one, 0)},
		"no object":        {fn(shape, nil, 0), fn(shape, one, 0)},
	}
	for name, pair := range pairs {
		a, b := pair[0], pair[1]
		if FormatTraceValue(a) != FormatTraceValue(b) {
			t.Fatalf("%s: %s and %s render apart", name, FormatTraceValue(a), FormatTraceValue(b))
		}
		if valueEqual(a, b) {
			t.Errorf("%s: %s compares equal to its twin", name, FormatValue(a))
		}
		c := canonicalCompare(a, b)
		if c == 0 || canonicalCompare(b, a) != -c {
			t.Errorf("%s: compare = %d, reversed = %d, want opposite non-zero", name, c, canonicalCompare(b, a))
		}
		if canonicalCompare(a, a) != 0 || canonicalCompare(b, fn(b.function().shape, b.FunctionSelf(), b.functionRun())) != 0 {
			t.Errorf("%s: a function compares non-zero against itself", name)
		}
		ab, ba := setOf([]Value{a, b}).Set(), setOf([]Value{b, a}).Set()
		if ab.Size() != 2 || !ab.Equal(ba) {
			t.Fatalf("%s: sets of %s and %s are not two equal members", name, FormatValue(a), FormatValue(b))
		}
		x, y := ab.Elements(), ba.Elements()
		for i := range x {
			if !valueEqual(x[i], y[i]) {
				t.Errorf("%s: element %d differs between insertion orders", name, i)
			}
		}
	}
}

// TestEqualSetsWithOtherRepresentativesEnumerateAlike pins that a member takes
// the place of the value it equals whichever kind carries it: a real-axis
// Complex sits among the numbers, `()` and `{}` with null, so two equal sets
// holding different representatives enumerate, render and cross the wire alike.
func TestEqualSetsWithOtherRepresentativesEnumerateAlike(t *testing.T) {
	ints := func(ns ...int64) []Value {
		vals := make([]Value, len(ns))
		for i, n := range ns {
			vals[i] = integerValue(n)
		}
		return vals
	}
	for _, tc := range []struct {
		name       string
		one, other []Value
	}{
		{"real-axis complex", []Value{integerValue(3), integerValue(2)}, []Value{integerValue(3), NewComplex(2)}},
		{"real-axis complex among reals", []Value{realConst(2.5), realConst(1.5)}, []Value{realConst(2.5), NewComplex(1.5)}},
		{"empty sequence", []Value{boolValue(true), {Kind: ValNull}}, []Value{boolValue(true), sequenceOf(nil)}},
		{"empty set", []Value{boolValue(true), {Kind: ValNull}}, []Value{boolValue(true), setOf(nil)}},
		{"nested", []Value{sequenceOf([]Value{{Kind: ValNull}, integerValue(2)}), sequenceOf([]Value{boolValue(true), integerValue(1)})},
			[]Value{sequenceOf([]Value{setOf(nil), integerValue(2)}), sequenceOf([]Value{boolValue(true), integerValue(1)})}},
		{"integer and real spelt apart", []Value{sequenceOf([]Value{integerValue(2)}), sequenceOf(ints(2, 1))},
			[]Value{sequenceOf([]Value{realConst(2)}), sequenceOf(ints(2, 1))}},
		{"complex element", []Value{sequenceOf(ints(10)), sequenceOf(ints(9))},
			[]Value{sequenceOf([]Value{NewComplex(10)}), sequenceOf(ints(9))}},
	} {
		one, other := setOf(tc.one).Set(), setOf(tc.other).Set()
		if !one.Equal(other) || one.Size() != 2 {
			t.Fatalf("%s: %s and %s are not equal sets of two", tc.name, FormatValue(setOf(tc.one)), FormatValue(setOf(tc.other)))
		}
		x, y := one.Elements(), other.Elements()
		for i := range x {
			if !valueEqual(x[i], y[i]) {
				t.Errorf("%s: element %d is %s in one set, %s in the other", tc.name, i, FormatValue(x[i]), FormatValue(y[i]))
			}
		}
		if !valueEqual(sequenceOf(x), sequenceOf(y)) {
			t.Errorf("%s: canonical sequences %s and %s differ", tc.name, FormatValue(sequenceOf(x)), FormatValue(sequenceOf(y)))
		}
		for _, elems := range [][]Value{tc.one, tc.other} {
			for i, j := 0, len(elems)-1; i < j; i, j = i+1, j-1 {
				elems[i], elems[j] = elems[j], elems[i]
			}
			if !valueEqual(sequenceOf(setOf(elems).Set().Elements()), sequenceOf(x)) {
				t.Errorf("%s: reversed insertion enumerates %s", tc.name, FormatValue(setOf(elems)))
			}
		}
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
