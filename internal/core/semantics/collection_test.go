package semantics

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// collectionModel builds a model of parts C with a mass and a name, held by S as
// cs [*], where the collection expressions under test are S's attribute values.
func collectionModel(t *testing.T, attributes string) (*Model, *symbols.Scope) {
	t.Helper()
	m, root := stdlibModelWithDoc(t, "collection.sysml", `package P {
		private import ScalarValues::*;
		private import ISQ::*;
		private import SI::*;
		private import ControlFunctions::*;
		private import ScalarFunctions::'+';
		part def C { attribute mass :> ISQ::mass; attribute name : String; attribute nothing : String[0]; }
		part def S {
			part cs : C[*];
			`+attributes+`
		}
	}`)
	return m, sym(t, sym(t, root, "P").Scope, "S").Scope
}

// valueOf is the value expression of the attribute named in scope.
func valueOf(t *testing.T, scope *symbols.Scope, name string) ast.Node {
	t.Helper()
	u, ok := sym(t, scope, name).Decl.(*ast.Usage)
	if !ok || u.Value == nil {
		t.Fatalf("%s: not a valued usage", name)
	}
	return u.Value
}

// typeNamesOf is the leaf names of ExprResultTypes of the attribute's value, and
// checks ExprResultType agrees with its first.
func typeNamesOf(t *testing.T, m *Model, scope *symbols.Scope, name string) []string {
	t.Helper()
	value := valueOf(t, scope, name)
	types := m.ExprResultTypes(scope, value)
	first := m.ExprResultType(scope, value)
	if len(types) == 0 {
		if first != nil {
			t.Fatalf("%s: ExprResultType = %s, ExprResultTypes none", name, first.Name)
		}
		return nil
	}
	if first != types[0] {
		t.Fatalf("%s: ExprResultType = %v, ExprResultTypes start with %v", name, first, types[0])
	}
	names := make([]string, 0, len(types))
	for _, typ := range types {
		names = append(names, leafName(typ.Name))
	}
	return names
}

func wantValueTypes(t *testing.T, m *Model, scope *symbols.Scope, name string, want ...string) {
	t.Helper()
	if got := typeNamesOf(t, m, scope, name); !slices.Equal(got, want) {
		t.Errorf("%s: result types %v, want %v", name, got, want)
	}
}

// wantResultRange checks the multiplicity of the result parameter of the function
// the attribute's value calls, which the specialized type does not narrow.
func wantResultRange(t *testing.T, m *Model, scope *symbols.Scope, name string, lower, upper int64, unbounded bool) {
	t.Helper()
	call, ok := valueOf(t, scope, name).(*ast.InvocationExpr)
	if !ok {
		t.Fatalf("%s: not an invocation", name)
	}
	r, ok := m.MultiplicityOf(m.invocationResult(scope, call))
	if !ok {
		t.Fatalf("%s: result parameter declares no multiplicity", name)
	}
	if !r.Lower.Known || r.Lower.Value != lower || !r.Upper.Known || r.Upper.Infinite != unbounded || (!unbounded && r.Upper.Value != upper) {
		t.Errorf("%s: result multiplicity %+v, want [%d..%d] unbounded=%v", name, r, lower, upper, unbounded)
	}
}

// collect results in what its body returns, in either notation, whatever the
// element type; select, reject and selectOne keep the elements of the collection.
func TestCollectionResultTypes(t *testing.T) {
	m, s := collectionModel(t, `
		attribute masses = cs->collect { in x : C; x.mass };
		attribute masses2 = cs.{ in x : C; x.mass };
		attribute names = cs->collect { in x : C; x.name };
		attribute heavy = cs->select { in x : C; x.mass > 1 [kg] };
		attribute heavy2 = cs.?{ in x : C; x.mass > 1 [kg] };
		attribute light = cs->reject { in x : C; x.mass > 1 [kg] };
		attribute one = cs->selectOne { in x : C; x.mass > 1 [kg] };
		attribute all = cs->forAll { in x : C; x.mass > 1 [kg] };
		attribute some = cs->exists { in x : C; x.mass > 1 [kg] };
		attribute total = cs->collect { in x : C; x.mass }->reduce '+';
		attribute total2 = cs->collect { in x : C; x.mass }->reduce { in a : MassValue; in b : MassValue; a };
		attribute plain = cs.mass;`)
	wantValueTypes(t, m, s, "masses", "MassValue")
	wantValueTypes(t, m, s, "masses2", "MassValue")
	wantValueTypes(t, m, s, "names", "String")
	wantValueTypes(t, m, s, "heavy", "C")
	wantValueTypes(t, m, s, "heavy2", "C")
	wantValueTypes(t, m, s, "light", "C")
	wantValueTypes(t, m, s, "one", "C")
	wantValueTypes(t, m, s, "all", "Boolean")
	wantValueTypes(t, m, s, "some", "Boolean")
	wantValueTypes(t, m, s, "total", "ScalarValue")
	wantValueTypes(t, m, s, "total2", "MassValue")
	wantValueTypes(t, m, s, "plain", "MassValue")
}

// The result multiplicity is the one the library declares for each function: a
// body returning one value per element still collects to `[0..*]`.
func TestCollectionResultMultiplicity(t *testing.T) {
	m, s := collectionModel(t, `
		attribute masses = cs->collect { in x : C; x.mass };
		attribute heavy = cs->select { in x : C; x.mass > 1 [kg] };
		attribute light = cs->reject { in x : C; x.mass > 1 [kg] };
		attribute one = cs->selectOne { in x : C; x.mass > 1 [kg] };
		attribute all = cs->forAll { in x : C; x.mass > 1 [kg] };
		attribute some = cs->exists { in x : C; x.mass > 1 [kg] };
		attribute total = cs->collect { in x : C; x.mass }->reduce '+';`)
	wantResultRange(t, m, s, "masses", 0, 0, true)
	wantResultRange(t, m, s, "heavy", 0, 0, true)
	wantResultRange(t, m, s, "light", 0, 0, true)
	wantResultRange(t, m, s, "one", 0, 1, false)
	wantResultRange(t, m, s, "all", 1, 1, false)
	wantResultRange(t, m, s, "some", 1, 1, false)
	wantResultRange(t, m, s, "total", 0, 0, true)
}

// A nested collect types the outer body by the inner result; a body returning a
// sequence is typed by what every element conforms to — the nearest supertype the
// elements share, Anything where an element is untyped.
func TestCollectionNestedAndSequenceBodies(t *testing.T) {
	m, s := collectionModel(t, `
		attribute nested = cs->collect { in x : C; cs->collect { in y : C; y.name } };
		attribute nested2 = cs.{ in x : C; cs.{ in y : C; y.mass } };
		attribute pairs = cs->collect { in x : C; (x.mass, x.mass) };
		attribute widened = cs->collect { in x : C; (1, 2.5) };
		attribute mixed = cs->collect { in x : C; (x.mass, (x.name, x.mass)) };
		attribute partly = cs->collect { in x; (x, 1) };
		attribute chained = cs->collect { in x : C; x.name }->select { in n : String; n == "a" };`)
	wantValueTypes(t, m, s, "nested", "String")
	wantValueTypes(t, m, s, "nested2", "MassValue")
	wantValueTypes(t, m, s, "pairs", "MassValue")
	wantValueTypes(t, m, s, "widened", "Real")
	wantValueTypes(t, m, s, "mixed", "ScalarValue")
	wantValueTypes(t, m, s, "partly", "Anything")
	wantValueTypes(t, m, s, "chained", "String")
}

// The arguments bind by the library's parameters in the prefix and named notations too.
func TestCollectionArgumentNotations(t *testing.T) {
	m, s := collectionModel(t, `
		attribute masses = collect(cs, { in x : C; x.mass });
		attribute named = collect(mapper = { in x : C; x.name }, collection = cs);
		attribute heavy = select(collection = cs, selector = { in x : C; x.mass > 1 [kg] });
		attribute fromFunction = cs->collect Mass;
		calc def Mass { in c : C; return : MassValue = c.mass; }`)
	wantValueTypes(t, m, s, "masses", "MassValue")
	wantValueTypes(t, m, s, "named", "String")
	wantValueTypes(t, m, s, "heavy", "C")
	wantValueTypes(t, m, s, "fromFunction", "MassValue")
}

// An untyped parameter of a body a collection function applies is bound to each element
// of the collection (KerML 8.3.4.8), so it is of the elements' type: `x.mass` resolves and
// types the body's result in every notation, the receiver, prefix and named ones alike.
func TestCollectionUntypedBodyParameterTakesElementType(t *testing.T) {
	m, s := collectionModel(t, `
		attribute masses = cs->collect { in x; x.mass };
		attribute masses2 = cs.{ in x; x.mass };
		attribute names = collect(cs, { in x; x.name });
		attribute named = collect(mapper = { in x; x.name }, collection = cs);
		attribute heavy = cs->select { in x; x.mass > 1 [kg] };
		attribute heavy2 = cs.?{ in x; x.mass > 1 [kg] };
		attribute heavy3 = select(selector = { in x; x.mass > 1 [kg] }, collection = cs);
		attribute light = cs->reject { in x; x.mass > 1 [kg] };
		attribute one = cs->selectOne { in x; x.mass > 1 [kg] };
		attribute all = cs->forAll { in x; x.mass > 1 [kg] };
		attribute some = cs->exists { in x; x.mass > 1 [kg] };
		attribute ints : Integer[*];
		attribute least = ints->minimize { in x; x + 1 };
		attribute most = ints->maximize { in x; x + 1 };
		attribute total = cs->collect { in x; x.mass }->reduce '+';`)
	wantValueTypes(t, m, s, "masses", "MassValue")
	wantValueTypes(t, m, s, "masses2", "MassValue")
	wantValueTypes(t, m, s, "names", "String")
	wantValueTypes(t, m, s, "named", "String")
	wantValueTypes(t, m, s, "heavy", "C")
	wantValueTypes(t, m, s, "heavy2", "C")
	wantValueTypes(t, m, s, "heavy3", "C")
	wantValueTypes(t, m, s, "light", "C")
	wantValueTypes(t, m, s, "one", "C")
	wantValueTypes(t, m, s, "all", "Boolean")
	wantValueTypes(t, m, s, "some", "Boolean")
	wantValueTypes(t, m, s, "least", "ScalarValue")
	wantValueTypes(t, m, s, "most", "ScalarValue")
	wantValueTypes(t, m, s, "total", "ScalarValue")
	for _, name := range []string{"masses", "masses2", "names", "named", "heavy", "heavy2", "heavy3", "light", "one", "all", "some", "least", "most"} {
		wantBodyParameterTypes(t, m, s, name, "x", elementTypeOf(name))
	}
}

// elementTypeOf is the element type the parameter x of the named attribute's body takes in
// TestCollectionUntypedBodyParameterTakesElementType: Integer over ints, C over cs.
func elementTypeOf(name string) string {
	if name == "least" || name == "most" {
		return "Integer"
	}
	return "C"
}

// wantBodyParameterTypes checks the types the named parameter of the body the attribute's
// value applies takes, through BodyParameterElementTypes and the specialization graph alike.
func wantBodyParameterTypes(t *testing.T, m *Model, scope *symbols.Scope, name, param string, want ...string) {
	t.Helper()
	body := appliedBody(t, valueOf(t, scope, name))
	p := sym(t, symbols.BodyExprScope(scope, body), param)
	var got, supers []string
	for _, typ := range m.BodyParameterElementTypes(p) {
		got = append(got, leafName(typ.Name))
	}
	for _, typ := range m.DirectSupertypes(p) {
		supers = append(supers, leafName(typ.Name))
	}
	if !slices.Equal(got, want) || !slices.Equal(supers, want) {
		t.Errorf("%s: parameter %s typed %v, supertypes %v, want %v", name, param, got, supers, want)
	}
}

// appliedBody is the body expression a collection value applies.
func appliedBody(t *testing.T, value ast.Node) *ast.BodyExpr {
	t.Helper()
	var arg ast.Node
	switch v := value.(type) {
	case *ast.CollectExpr:
		arg = v.Body
	case *ast.SelectExpr:
		arg = v.Body
	case *ast.InvocationExpr:
		for _, a := range v.Args {
			if _, ok := a.(*ast.BodyExpr); ok {
				arg = a
			}
		}
		for _, a := range v.NamedArgs {
			if _, ok := a.Value.(*ast.BodyExpr); ok {
				arg = a.Value
			}
		}
	}
	body, ok := arg.(*ast.BodyExpr)
	if !ok {
		t.Fatalf("value %T applies no body", value)
	}
	return body
}

// A reducer's second parameter takes the elements' type; its first, which holds the reducer's
// result from the second fold on, only where that result conforms to it.
func TestCollectionReduceUntypedParametersTakeElementType(t *testing.T) {
	m, s := collectionModel(t, `
		part def D :> C { part next : D[0..1]; }
		part ds : D[*];
		part one : C;
		attribute either = cs->reduce { in a; in b; a };
		attribute other = cs->reduce { in a; in b; b };
		attribute named = reduce(reducer = { in a; in b; one }, collection = cs);
		attribute special = cs->reduce { in a; in b; ds#(1) };
		attribute chained = ds->reduce { in a; in b; a.next };
		attribute total = cs->reduce { in a; in b; a.mass };
		attribute label = reduce(reducer = { in a; in b; b.name }, collection = cs);
		attribute heaviest = cs->reduce { in a; in b; if a.mass > b.mass ? a else b };
		attribute general = ds->reduce { in a; in b; one };`)
	wantValueTypes(t, m, s, "either", "C")
	wantValueTypes(t, m, s, "chained", "D")
	for _, name := range []string{"either", "other", "named", "special"} {
		wantBodyParameterTypes(t, m, s, name, "a", "C")
		wantBodyParameterTypes(t, m, s, name, "b", "C")
	}
	wantBodyParameterTypes(t, m, s, "chained", "a", "D")
	wantBodyParameterTypes(t, m, s, "chained", "b", "D")
	for _, name := range []string{"total", "label", "heaviest"} {
		wantBodyParameterTypes(t, m, s, name, "a")
		wantBodyParameterTypes(t, m, s, name, "b", "C")
	}
	wantBodyParameterTypes(t, m, s, "general", "a")
	wantBodyParameterTypes(t, m, s, "general", "b", "D")
	if c := m.ExprConformsToLibrary(s, valueOf(t, s, "heaviest"), fqnString); !c.Known || c.Holds || c.Untyped || c.Found != "C" {
		t.Errorf("heaviest as String: %+v, want known, not holding, found C", c)
	}
	elements, ok := m.CollectionElements(s, valueOf(t, s, "label"))
	if !ok || len(elements) != 2 || len(elements[0].Types) != 1 || leafName(elements[0].Types[0].Name) != "String" ||
		len(elements[1].Types) != 1 || leafName(elements[1].Types[0].Name) != "C" {
		t.Errorf("label: elements %v, want the reducer's String then the collection's C", elements)
	}
}

// The guard's decision is memoized as the first parameter's supertypes, never the assumption.
func TestCollectionReduceGuardMemoizesDecision(t *testing.T) {
	m, s := collectionModel(t, `
		attribute total = cs->reduce { in a; in b; a.mass };
		attribute either = cs->reduce { in a; in b; a };`)
	for i := 0; i < 2; i++ {
		wantBodyParameterTypes(t, m, s, "total", "a")
		wantBodyParameterTypes(t, m, s, "either", "a", "C")
		wantValueTypes(t, m, s, "total", "Anything")
		wantValueTypes(t, m, s, "either", "C")
	}
	a := sym(t, symbols.BodyExprScope(s, appliedBody(t, valueOf(t, s, "total"))), "a")
	if len(m.AllSupertypes(a)) != 0 || len(m.MembersOf(a)) != 0 {
		t.Errorf("total: a has supertypes %v and members %v after the guard, want none", m.AllSupertypes(a), m.MembersOf(a))
	}
}

// A nested body's parameter is bound to the elements of the collection its own operation is
// applied to, which the outer parameter's type decides: `x.parts` holds Cs, so `p` is a C.
func TestCollectionNestedUntypedBodyParameters(t *testing.T) {
	m, s := collectionModel(t, `
		part def Assembly { part parts : C[*]; }
		part assemblies : Assembly[*];
		attribute nested = assemblies.{ in x; x.parts.{ in p; p.mass } };
		attribute nested2 = assemblies->collect { in x; x.parts->select { in p; p.mass > 1 [kg] } };
		attribute nested3 = assemblies->forAll { in x; x.parts->exists { in p; p.mass > 1 [kg] } };`)
	wantValueTypes(t, m, s, "nested", "MassValue")
	wantValueTypes(t, m, s, "nested2", "C")
	wantValueTypes(t, m, s, "nested3", "Boolean")
	wantBodyParameterTypes(t, m, s, "nested", "x", "Assembly")
	outer := appliedBody(t, valueOf(t, s, "nested"))
	inner := appliedBody(t, outer.Result)
	p := sym(t, symbols.BodyExprScope(symbols.BodyExprScope(s, outer), inner), "p")
	if types := m.BodyParameterElementTypes(p); len(types) != 1 || leafName(types[0].Name) != "C" {
		t.Errorf("nested: inner parameter p typed %v, want C", types)
	}
}

// A body whose parameter declares its own type keeps it, whatever the elements; one applied
// over a collection whose elements cannot be typed stays untyped — the library's Anything, not
// a guess — as does a body that is not the argument of a collection function.
func TestCollectionUntypedBodyFallsBackToLibraryResult(t *testing.T) {
	m, s := collectionModel(t, `
		attribute anys;
		attribute unknown = anys->collect { in x; x.mass };
		attribute unknown2 = anys.{ in x; x.mass };
		attribute declared = cs->collect { in x : String; x };
		attribute body = cs->collect { in x : C; { in y; y } };`)
	wantValueTypes(t, m, s, "unknown", "Anything")
	wantValueTypes(t, m, s, "unknown2", "Anything")
	wantValueTypes(t, m, s, "declared", "String")
	wantValueTypes(t, m, s, "body", "Evaluation")
	wantBodyParameterTypes(t, m, s, "unknown", "x")
	wantBodyParameterTypes(t, m, s, "unknown2", "x")
	if got := m.BodyParameterElementTypes(sym(t, symbols.BodyExprScope(s, appliedBody(t, valueOf(t, s, "declared"))), "x")); got != nil {
		t.Errorf("declared: element types %v for a parameter declaring its type, want none", got)
	}
}

// A body that names the feature it values leads the typer back to itself, as does an untyped
// parameter over the feature the body values; typing terminates.
func TestCollectionSelfReferentialBodyTerminates(t *testing.T) {
	m, s := collectionModel(t, `
		attribute total :> ISQ::mass = cs->collect { in x : C; total }->reduce '+';
		attribute loop = cs->collect { in x : C; loop };
		attribute loop2 = cs.{ in x : C; loop2 };
		attribute loop3 = loop3.{ in x; x.mass };
		attribute loop4 = cs.{ in x; (x, loop4) };
		attribute loop5 = loop5->reduce { in a; in b; a };
		attribute kept = cs->select { in x : C; kept == x };
		attribute kept2 = cs->select { in x; kept2 == x };`)
	wantValueTypes(t, m, s, "total", "ScalarValue")
	wantValueTypes(t, m, s, "loop", "Anything")
	wantValueTypes(t, m, s, "loop2", "Anything")
	wantValueTypes(t, m, s, "loop3", "Anything")
	wantValueTypes(t, m, s, "loop4", "Anything")
	wantValueTypes(t, m, s, "loop5", "Anything")
	wantValueTypes(t, m, s, "kept", "C")
	wantValueTypes(t, m, s, "kept2", "C")
	wantBodyParameterTypes(t, m, s, "loop3", "x")
	wantBodyParameterTypes(t, m, s, "loop4", "x", "C")
	wantBodyParameterTypes(t, m, s, "loop5", "a")
	wantBodyParameterTypes(t, m, s, "kept2", "x", "C")
}

// Conformance judges the specialized result: a collect of masses is a MassValue and
// no String; a select keeps C, no ScalarValue; an untyped body stays untyped. A body
// returning a sequence conforms when every element does, fails naming an element
// known not to, and stays the untyped Anything while an element is untyped and none fails.
func TestCollectionResultConformance(t *testing.T) {
	m, s := collectionModel(t, `
		attribute masses = cs->collect { in x : C; x.mass };
		attribute masses2 = cs.{ in x : C; x.mass };
		attribute heavy = cs->select { in x : C; x.mass > 1 [kg] };
		attribute anys;
		attribute unknown = anys.{ in x; x.mass };
		attribute pairs = cs->collect { in x : C; (x.mass, x.mass) };
		attribute mixed = cs.{ in x : C; (true, 1) };
		attribute partly = anys.{ in x; (x, 1) };
		attribute partly2 = anys->collect { in x; (x, 1) };`)
	for name, want := range map[string]string{"pairs": "ISQ::MassValue", "mixed": "Base::Anything"} {
		if c := m.ExprConformsToLibrary(s, valueOf(t, s, name), want); !c.Known || !c.Holds {
			t.Errorf("%s as %s: %+v, want it to hold", name, want, c)
		}
	}
	if c := m.ExprConformsToLibrary(s, valueOf(t, s, "pairs"), fqnString); !c.Known || c.Holds || c.Untyped || c.Found != "MassValue" {
		t.Errorf("pairs as String: %+v, want known, not holding, found MassValue", c)
	}
	for _, want := range []string{FQNBoolean, fqnInteger} {
		if c := m.ExprConformsToLibrary(s, valueOf(t, s, "mixed"), want); !c.Known || c.Holds || c.Untyped || c.Found == "" {
			t.Errorf("mixed as %s: %+v, want known, not holding, naming the element", want, c)
		}
	}
	if c := m.ExprConformsToLibrary(s, valueOf(t, s, "partly"), fqnInteger); !c.Known || c.Holds || !c.Untyped || !strings.Contains(c.Found, "Anything") {
		t.Errorf("partly as Integer: %+v, want untyped Anything", c)
	}
	for _, name := range []string{"partly", "partly2"} {
		if c := m.ExprConformsToLibrary(s, valueOf(t, s, name), FQNBoolean); !c.Known || c.Holds || c.Untyped || c.Found != "Natural" {
			t.Errorf("%s as Boolean: %+v, want known, not holding, found Natural", name, c)
		}
	}
	for _, name := range []string{"masses", "masses2"} {
		c := m.ExprConformsToLibrary(s, valueOf(t, s, name), "ISQ::MassValue")
		if !c.Known || !c.Holds {
			t.Errorf("%s as MassValue: %+v, want it to hold", name, c)
		}
		c = m.ExprConformsToLibrary(s, valueOf(t, s, name), fqnString)
		if !c.Known || c.Holds || c.Untyped || c.Found != "MassValue" {
			t.Errorf("%s as String: %+v, want known, not holding, found MassValue", name, c)
		}
	}
	c := m.ExprConformsToLibrary(s, valueOf(t, s, "heavy"), "ScalarValues::ScalarValue")
	if !c.Known || c.Holds || c.Found != "C" {
		t.Errorf("heavy as ScalarValue: %+v, want known, not holding, found C", c)
	}
	c = m.ExprConformsToLibrary(s, valueOf(t, s, "unknown"), fqnString)
	if !c.Known || c.Holds || !c.Untyped || !strings.Contains(c.Found, "Anything") {
		t.Errorf("unknown as String: %+v, want untyped Anything", c)
	}
}

// reduce returns what its reducer does, or the collection's one element unreduced, so
// the result is what both conform to — the library's Anything where they share nothing —
// unless the collection holds two or more: a declared `[2..*]`, one inherited by
// redefinition, a chain through such a feature, or a sequence of two literals; a collection
// holding one at most, `[1]` or `[0..1]`, is never reduced, so its element alone is the result.
func TestCollectionReduceMayReturnTheElement(t *testing.T) {
	m, s := collectionModel(t, `
		part two : C[2..*];
		part one : C[1];
		part atMost : C[0..1];
		part def Pair { part items : C[2]; part item : C[1]; }
		part def Pairs :> Pair { part :>> items; part :>> item; }
		part pair : Pairs[1];
		part pairs : Pairs[*];
		part couple : Pairs[2];
		attribute names = cs->reduce { in a : C; in b : C; a.name };
		attribute names1 = one->reduce { in a : C; in b : C; a.name };
		attribute names01 = atMost->reduce { in a : C; in b : C; a.name };
		attribute names1lit = (one)->reduce { in a : C; in b : C; "s" };
		attribute names2 = two->reduce { in a : C; in b : C; a.name };
		attribute inherited = pair.items->reduce { in a : C; in b : C; a.name };
		attribute chained = couple.item->reduce { in a : C; in b : C; a.name };
		attribute chained1 = pair.item->reduce { in a : C; in b : C; a.name };
		attribute chainedAny = pairs.items->reduce { in a : C; in b : C; a.name };
		attribute lits = (1, 2)->reduce { in a : Integer; in b : Integer; "s" };
		attribute same = cs->reduce { in a : C; in b : C; a };
		attribute cast = cs->reduce { in a : C; in b : C; a.name } as String;
		attribute cast2 = two->reduce { in a : C; in b : C; a.name } as C;
		attribute cast1 = one->reduce { in a : C; in b : C; a.name } as String;`)
	wantValueTypes(t, m, s, "names", "Anything")
	wantValueTypes(t, m, s, "names1", "C")
	wantValueTypes(t, m, s, "names01", "C")
	wantValueTypes(t, m, s, "names1lit", "C")
	wantValueTypes(t, m, s, "names2", "String")
	wantValueTypes(t, m, s, "inherited", "String")
	wantValueTypes(t, m, s, "chained", "String")
	wantValueTypes(t, m, s, "chained1", "C")
	wantValueTypes(t, m, s, "chainedAny", "Anything")
	wantValueTypes(t, m, s, "lits", "String")
	wantValueTypes(t, m, s, "same", "C")
	elements, ok := m.CollectionElements(s, valueOf(t, s, "names"))
	if !ok || len(elements) != 2 || len(elements[0].Types) != 1 || len(elements[1].Types) != 1 ||
		leafName(elements[0].Types[0].Name) != "String" || leafName(elements[1].Types[0].Name) != "C" ||
		elements[0].Node == nil || elements[1].Node == nil {
		t.Errorf("names: elements %v, want the reducer's String then the collection's C", elements)
	}
	for _, name := range []string{"names", "names1", "names01"} {
		if c := m.ExprConformsToLibrary(s, valueOf(t, s, name), fqnString); !c.Known || c.Holds || c.Untyped || c.Found != "C" {
			t.Errorf("%s as String: %+v, want known, not holding, found C", name, c)
		}
	}
	for _, name := range []string{"names1", "names01"} {
		elements, ok := m.CollectionElements(s, valueOf(t, s, name))
		if !ok || len(elements) != 1 || len(elements[0].Types) != 1 || leafName(elements[0].Types[0].Name) != "C" {
			t.Errorf("%s: elements %v, want the collection's C alone", name, elements)
		}
	}
	if c := m.ExprConformsToLibrary(s, valueOf(t, s, "names2"), fqnString); !c.Known || !c.Holds {
		t.Errorf("names2 as String: %+v, want it to hold", c)
	}
	if c := m.CastConformance(s, valueOf(t, s, "cast").(*ast.OperatorExpr)); !c.Known || !c.Holds {
		t.Errorf("cast to String: %+v, want it to hold, the element may be one", c)
	}
	if c := m.CastConformance(s, valueOf(t, s, "cast2").(*ast.OperatorExpr)); !c.Known || c.Holds || c.Found != "String" {
		t.Errorf("cast of two to C: %+v, want known, not holding, found String", c)
	}
	if c := m.CastConformance(s, valueOf(t, s, "cast1").(*ast.OperatorExpr)); !c.Known || c.Holds || c.Found != "C" {
		t.Errorf("cast of one to String: %+v, want known, not holding, found C", c)
	}
}

// Over a collection known to hold nothing — or mapping each element to nothing — reduce is typed
// by the reducer alone and collect by its body; but no element is held, judged or refused a cast.
func TestCollectionReduceOfNothing(t *testing.T) {
	m, s := collectionModel(t, `
		part none : C[0];
		part one : C[1];
		part def Holder { part item : C[1]; }
		part holders : Holder[0..0];
		attribute empty = ()->reduce { in a : Integer; in b : Integer; "s" };
		attribute empty2 = none->reduce { in a : C; in b : C; a.name };
		attribute empty3 = holders.item->reduce { in a : C; in b : C; a.name };
		attribute emptyCollect = ()->collect { in a : Integer; "s" };
		attribute emptyCollect2 = none.{ in a : C; a.name };
		attribute emptyCollect3 = holders.item->collect { in a : C; a.name };
		attribute emptyBody = cs.{ in x : C; x.nothing };
		attribute emptyBody2 = cs->collect { in x : C; (x.nothing, x.nothing) };
		attribute emptyNested = (().{ in a : Integer; a }).{ in b : Integer; "s" };
		attribute emptyNested2 = (none->select { in a : C; true })->reduce { in a : C; in b : C; "s" };
		attribute emptyNested3 = (cs.{ in x : C; x.nothing }).?{ in s : String; true };
		attribute emptyNested4 = (cs->selectOne { in x : C; true }).{ in x : C; x.nothing }->collect { in s : String; s };
		attribute cast = none->reduce { in a : C; in b : C; a.name } as Integer;
		attribute cast2 = cs.{ in x : C; x.nothing } as Integer;
		attribute two = (none, one, one)->reduce { in a : C; in b : C; a.name };
		attribute maybe = (none, one)->reduce { in a : C; in b : C; a.name };`)
	for _, name := range []string{"empty", "empty2", "empty3", "emptyCollect", "emptyCollect2", "emptyCollect3",
		"emptyBody", "emptyBody2", "emptyNested", "emptyNested2", "emptyNested3", "emptyNested4"} {
		wantValueTypes(t, m, s, name, "String")
		elements, ok := m.CollectionElements(s, valueOf(t, s, name))
		if !ok || len(elements) != 0 {
			t.Errorf("%s: elements %v, want none", name, elements)
		}
		if types, ok := m.CollectionHeldTypes(s, valueOf(t, s, name)); !ok || len(types) != 0 {
			t.Errorf("%s: held types %v, want none", name, types)
		}
		for _, want := range []string{fqnString, fqnInteger} {
			if c := m.ExprConformsToLibrary(s, valueOf(t, s, name), want); c.Known && !c.Untyped {
				t.Errorf("%s as %s: %+v, want nothing decided", name, want, c)
			}
		}
	}
	for _, name := range []string{"cast", "cast2"} {
		if c := m.CastConformance(s, valueOf(t, s, name).(*ast.OperatorExpr)); c.Known && !c.Holds {
			t.Errorf("%s of nothing to Integer: %+v, want it not to fail", name, c)
		}
	}
	wantValueTypes(t, m, s, "two", "String")
	wantValueTypes(t, m, s, "maybe", "C")
	if types, ok := m.CollectionHeldTypes(s, valueOf(t, s, "two")); !ok || len(types) != 1 || leafName(types[0].Name) != "String" {
		t.Errorf("two: held types %v, want String", types)
	}
}

// How many values a feature or a mapper's result holds is read through an alias and, where
// it declares no multiplicity, from the feature it redefines by clause or by position.
func TestCollectionSizeThroughAliasAndRedefinition(t *testing.T) {
	m, s := collectionModel(t, `
		part none : C[0];
		part two : C[2];
		alias nobody for none;
		alias pair for two;
		function Nobody { in c : C; return r : C[0]; }
		function Nobody2 :> Nobody { in c : C; return r :>> r; }
		function Nobody3 :> Nobody { in c : C; return r : C; }
		function Named { in c : C; return r : String; }
		attribute viaAlias = nobody.{ in c : C; c.name };
		attribute viaAlias2 = pair->reduce { in a : C; in b : C; a.name };
		attribute inherited = cs->collect Nobody2;
		attribute inherited2 = (cs->collect Nobody2)->reduce { in a : C; in b : C; a.name };
		attribute implicit = cs->collect Nobody3;
		attribute implicit2 = (cs->collect Nobody3)->reduce { in a : C; in b : C; a.name };
		attribute named = cs->collect Named;`)
	for _, name := range []string{"viaAlias", "inherited2", "implicit2"} {
		wantValueTypes(t, m, s, name, "String")
		if elements, ok := m.CollectionElements(s, valueOf(t, s, name)); !ok || len(elements) != 0 {
			t.Errorf("%s: elements %v, want none", name, elements)
		}
	}
	wantValueTypes(t, m, s, "viaAlias2", "String")
	if elements, ok := m.CollectionElements(s, valueOf(t, s, "viaAlias2")); !ok || len(elements) != 1 || leafName(elements[0].Types[0].Name) != "String" {
		t.Errorf("viaAlias2: elements %v, want the reducer's String alone", elements)
	}
	for _, name := range []string{"inherited", "implicit"} {
		wantValueTypes(t, m, s, name, "C")
		if elements, ok := m.CollectionElements(s, valueOf(t, s, name)); !ok || len(elements) != 0 {
			t.Errorf("%s: elements %v, want none", name, elements)
		}
	}
	if elements, ok := m.CollectionElements(s, valueOf(t, s, "named")); !ok || len(elements) != 1 {
		t.Errorf("named: elements %v, want the result parameter", elements)
	}
}

// CollectionValues sizes a collection value alone — an operation over a known collection — and
// says nothing of a literal or a feature, whose sizes are read elsewhere. A reduce yields the one
// element it holds unreduced, or what its reducer yields over two or more, or either.
func TestCollectionValues(t *testing.T) {
	m, s := collectionModel(t, `
		part none : C[0];
		part one : C[1];
		part maybe : C[0..1];
		part two : C[2];
		function Pair { in a : C; in b : C; return r : C[2]; }
		attribute mapped = two.{ in c : C; (c.name, c.name) };
		attribute reduced = two->reduce { in a : C; in b : C; a.name };
		attribute reducedTwo = two->reduce { in a : C; in b : C; (a, b) };
		attribute reducedNone = two->reduce { in a : C; in b : C; () };
		attribute reducedNamed = two->reduce Pair;
		attribute reducedOpen = two->reduce { in a : C; in b : C; a.name + b.name };
		attribute reducedOne = one->reduce { in a : C; in b : C; (a, b) };
		attribute reducedMaybe = maybe->reduce { in a : C; in b : C; (a, b) };
		attribute reducedAny = cs->reduce { in a : C; in b : C; (a, b) };
		attribute reducedEmpty = none->reduce { in a : C; in b : C; (a, b) };
		attribute nothing = none->select { in c : C; true };
		attribute atMost = two->selectOne { in c : C; true };
		attribute open = cs.{ in c : C; c.name };
		attribute literal = (1, 2);
		attribute named = two;`)
	for name, want := range map[string]string{
		"mapped": "[4]", "reduced": "[1]", "nothing": "[0]", "atMost": "[0..1]", "open": "[0..*]",
		"reducedTwo": "[2]", "reducedNone": "[0]", "reducedNamed": "[2]", "reducedOpen": "[0..*]",
		"reducedOne": "[1]", "reducedMaybe": "[0..1]", "reducedAny": "[0..2]", "reducedEmpty": "[0]",
	} {
		r, ok := m.CollectionValues(s, valueOf(t, s, name))
		if !ok || r.Text() != want {
			t.Errorf("%s: %s %v, want %s", name, r.Text(), ok, want)
		}
	}
	for _, name := range []string{"literal", "named"} {
		if _, ok := m.CollectionValues(s, valueOf(t, s, name)); ok {
			t.Errorf("%s: sized as a collection value", name)
		}
	}
	if n, ok := CountRange(3).Exactly(); !ok || n != 3 {
		t.Errorf("[3] exactly %d %v, want 3", n, ok)
	}
	atLeastOne := Range{Lower: Bound{Value: 1, Known: true}, Upper: unbounded}
	if _, ok := atLeastOne.Exactly(); ok {
		t.Error("[1..*] is exact")
	}
	if sum := CountRange(math.MaxInt64).Plus(CountRange(1)); !sum.Lower.Infinite || !sum.Upper.Infinite {
		t.Errorf("[%d] + [1] is %s, want [*..*]", int64(math.MaxInt64), sum.Text())
	}
	one := CountRange(1)
	for _, c := range []struct{ held, want string }{
		{"[2]", "2 value(s) bound to a feature with multiplicity upper bound 1"},
		{"[0]", "0 value(s) bound to a feature with multiplicity lower bound 1"},
		{"[2..*]", "at least 2 value(s) bound to a feature with multiplicity upper bound 1"},
		{"[*..*]", "more than 9223372036854775807 value(s) bound to a feature with multiplicity upper bound 1"},
		{"[0..*]", ""},
		{"[0..1]", ""},
	} {
		held := map[string]Range{
			"[2]": CountRange(2), "[0]": CountRange(0), "[0..1]": {Lower: CountRange(0).Lower, Upper: one.Upper},
			"[2..*]": {Lower: CountRange(2).Lower, Upper: unbounded}, "[*..*]": {Lower: unbounded, Upper: unbounded},
			"[0..*]": {Lower: CountRange(0).Lower, Upper: unbounded},
		}[c.held]
		if got := one.HeldViolation(held); got != c.want {
			t.Errorf("%s held by [1]: %q, want %q", c.held, got, c.want)
		}
	}
	if got := CountRange(3).HeldViolation(Range{Lower: CountRange(0).Lower, Upper: CountRange(2).Upper}); got != "at most 2 value(s) bound to a feature with multiplicity lower bound 3" {
		t.Errorf("[0..2] held by [3]: %q", got)
	}
}

// `xs.?{…}` is select written out: its elements are the collection's, so a binding or an
// argument is judged by them as `xs->select {…}` is, and a sequence written out is typed by
// what its elements share rather than the Anything the sequence itself is.
func TestCollectionSelectShorthandElements(t *testing.T) {
	m, s := collectionModel(t, `
		part c1 : C;
		part c2 : C;
		attribute heavy = cs.?{ in x : C; x.mass > 1 [kg] };
		attribute heavy2 = cs->select { in x : C; x.mass > 1 [kg] };
		attribute picked = (c1, c2).?{ in x : C; true };
		attribute picked2 = (c1, c2)->select { in x : C; true };
		attribute mixed = (c1, "s").?{ in x; true };
		attribute lits = (1, 2.5).?{ in x : Real; true };`)
	for _, name := range []string{"heavy", "heavy2"} {
		wantValueTypes(t, m, s, name, "C")
		elements, ok := m.CollectionElements(s, valueOf(t, s, name))
		if !ok || len(elements) != 1 || len(elements[0].Types) != 1 || leafName(elements[0].Types[0].Name) != "C" || elements[0].Node == nil {
			t.Errorf("%s: elements %v, want the collection's C", name, elements)
		}
	}
	for _, name := range []string{"picked", "picked2"} {
		wantValueTypes(t, m, s, name, "C")
	}
	wantValueTypes(t, m, s, "mixed", "Anything")
	wantValueTypes(t, m, s, "lits", "Real")
	elements, ok := m.CollectionElements(s, valueOf(t, s, "lits"))
	if !ok || len(elements) != 2 {
		t.Fatalf("lits: elements %v, want one per literal", elements)
	}
	if _, isInt := elements[0].Node.(*ast.LiteralInteger); !isInt {
		t.Errorf("lits: first element %T, want the literal 1", elements[0].Node)
	}
	if _, isReal := elements[1].Node.(*ast.LiteralReal); !isReal {
		t.Errorf("lits: second element %T, want the literal 2.5", elements[1].Node)
	}
}

// Elements of sibling types share their nearest common supertype, not Anything: a selection or
// a body keeping a Truck and a Car is a collection of Vehicles; one keeping a Truck and a
// String shares nothing worth saying.
func TestCollectionSiblingElementsShareSupertype(t *testing.T) {
	m, s := collectionModel(t, `
		part def T :> C; part def U :> C; part def V :> U;
		part t : T; part u : U; part v : V;
		attribute kin = (t, u).?{ in x : C; true };
		attribute kin2 = (t, u)->select { in x : C; true };
		attribute kin3 = cs.{ in x : C; (t, v) };
		attribute kin4 = (t, u, v)->reject { in x : C; false };
		attribute line = (u, v).?{ in x : C; true };
		attribute apart = (t, "s").?{ in x; true };`)
	for _, name := range []string{"kin", "kin2", "kin3", "kin4"} {
		wantValueTypes(t, m, s, name, "C")
	}
	wantValueTypes(t, m, s, "line", "U")
	wantValueTypes(t, m, s, "apart", "Anything")
}

// A union conforms to whatever all its unioning types do, so elements typed by a union and by a
// sibling share the unioning types' supertype whichever comes first, as do two such unions; a
// union whose members share nothing with the sibling stays Anything.
func TestCollectionUnionElementsShareSupertype(t *testing.T) {
	m, s := collectionModel(t, `
		part def T :> C; part def U :> C; part def V :> C;
		classifier TU unions T, U;
		classifier UV unions U, V;
		classifier TS unions T, String;
		part tu : TU; part uv : UV; part ts : TS; part v : V;
		attribute unionFirst = (tu, v).?{ in x; true };
		attribute unionLast = (v, tu).?{ in x; true };
		attribute unions = (tu, uv)->select { in x; true };
		attribute unions2 = cs.{ in x : C; (uv, tu) };
		attribute apart = (ts, v).?{ in x; true };`)
	for _, name := range []string{"unionFirst", "unionLast", "unions", "unions2"} {
		wantValueTypes(t, m, s, name, "C")
	}
	wantValueTypes(t, m, s, "apart", "Anything")
}

// An element that is itself a collection value contributes the elements it holds: a nested
// collect's literals, a selection's elements, none from one over nothing.
func TestCollectionNestedElements(t *testing.T) {
	m, s := collectionModel(t, `
		attribute nested = cs.{ in x : C; cs.{ in y : C; 1.5 } };
		attribute nested2 = cs->collect { in x : C; (x.name, cs->select { in y : C; true }) };
		attribute selected = (cs.{ in x : C; 2 }).?{ in n : Integer; true };
		attribute nestedEmpty = cs.{ in x : C; ().{ in y : Integer; 1.5 } };`)
	wantValueTypes(t, m, s, "nested", "Real")
	elements, ok := m.CollectionElements(s, valueOf(t, s, "nested"))
	if !ok || len(elements) != 1 {
		t.Fatalf("nested: elements %v, want the inner literal", elements)
	}
	if _, isReal := elements[0].Node.(*ast.LiteralReal); !isReal {
		t.Errorf("nested: element %T, want the literal 1.5", elements[0].Node)
	}
	elements, ok = m.CollectionElements(s, valueOf(t, s, "nested2"))
	if !ok || len(elements) != 2 {
		t.Fatalf("nested2: elements %v, want the name and the selection's C", elements)
	}
	if _, isRef := elements[0].Node.(*ast.FeatureChainExpr); !isRef || len(elements[0].Types) != 1 || leafName(elements[0].Types[0].Name) != "String" {
		t.Errorf("nested2: first element %T %v, want x.name : String", elements[0].Node, elements[0].Types)
	}
	if len(elements[1].Types) != 1 || leafName(elements[1].Types[0].Name) != "C" {
		t.Errorf("nested2: second element %v, want the selection's C", elements[1].Types)
	}
	elements, ok = m.CollectionElements(s, valueOf(t, s, "selected"))
	if !ok || len(elements) != 1 {
		t.Fatalf("selected: elements %v, want the collected literal", elements)
	}
	if _, isInt := elements[0].Node.(*ast.LiteralInteger); !isInt {
		t.Errorf("selected: element %T, want the literal 2", elements[0].Node)
	}
	wantValueTypes(t, m, s, "nestedEmpty", "Real")
	if elements, ok := m.CollectionElements(s, valueOf(t, s, "nestedEmpty")); !ok || len(elements) != 0 {
		t.Errorf("nestedEmpty: elements %v, want none", elements)
	}
}

// A mapper named by function holds one value per element where its result declares no
// multiplicity, as a feature does — not any number — while a result whose declared bound is not
// evaluable holds an unknown number; a count past int64 exceeds every bound.
func TestCollectionSizeThroughNamedMapperAndPastInt64(t *testing.T) {
	m, s := collectionModel(t, `
		part two : C[2];
		attribute n : Positive;
		function Named { in c : C; return r : String; }
		function Unsure { in c : C; return r : String[n]; }
		part def H { part many : C[9223372036854775807]; }
		part hs : H[2];
		part huge : C[9223372036854775807];
		attribute named = (two->collect Named)->reduce { in a : String; in b : String; 1 };
		attribute unsure = (two->collect Unsure)->reduce { in a : String; in b : String; 1 };
		attribute sum = (huge, huge)->reduce { in a : C; in b : C; a.name };
		attribute product = hs.many->reduce { in a : C; in b : C; a.name };`)
	wantValueTypes(t, m, s, "named", "Integer")
	if elements, ok := m.CollectionElements(s, valueOf(t, s, "named")); !ok || len(elements) != 1 || leafName(elements[0].Types[0].Name) != "Integer" {
		t.Errorf("named: elements %v, want the reducer's Integer alone", elements)
	}
	wantValueTypes(t, m, s, "unsure", "ScalarValue")
	if elements, ok := m.CollectionElements(s, valueOf(t, s, "unsure")); !ok || len(elements) != 2 {
		t.Errorf("unsure: elements %v, want the reducer's Integer and the mapper's String", elements)
	}
	for _, name := range []string{"sum", "product"} {
		wantValueTypes(t, m, s, name, "String")
		if elements, ok := m.CollectionElements(s, valueOf(t, s, name)); !ok || len(elements) != 1 || leafName(elements[0].Types[0].Name) != "String" {
			t.Errorf("%s: elements %v, want the reducer's String alone", name, elements)
		}
		collection := valueOf(t, s, name).(*ast.InvocationExpr).Operand
		r, ok := m.valuesHeldBy(s, collection)
		if !ok || !r.Lower.Known || !r.Lower.Infinite || !r.Upper.Known || !r.Upper.Infinite {
			t.Errorf("%s: values held %s, want [*..*]", name, r.Text())
		}
	}
}

// A cast holds once any element it may hold specializes the target, whichever comes first: an
// untyped element before an Integer leaves the Integer to decide; one beside a String leaves the
// cast undecided, and none at all names every element refused.
func TestCollectionCastDecidedByAnyElement(t *testing.T) {
	holds := Conformance{Known: true, Holds: true}
	if c := anyHolds([]Conformance{conformanceUnknown(), holds}); !c.Known || !c.Holds {
		t.Errorf("unknown then holding: %+v, want it to hold", c)
	}
	if c := anyHolds([]Conformance{{Known: true, Found: "String"}, conformanceUnknown()}); c.Known {
		t.Errorf("refused then unknown: %+v, want unknown", c)
	}
	if c := anyHolds([]Conformance{{Known: true, Found: "String"}, {Known: true, Found: "C"}}); !c.Known || c.Holds || c.Found != "String and C" {
		t.Errorf("refused twice: %+v, want known, not holding, found String and C", c)
	}
	m, s := collectionModel(t, `
		attribute loose;
		attribute cast = cs.{ in x : C; (loose, 1) } as Integer;
		attribute cast2 = cs.{ in x : C; (x.name, "s") } as Integer;`)
	if c := m.CastConformance(s, valueOf(t, s, "cast").(*ast.OperatorExpr)); !c.Known || !c.Holds {
		t.Errorf("cast to Integer: %+v, want it to hold by the literal 1", c)
	}
	if c := m.CastConformance(s, valueOf(t, s, "cast2").(*ast.OperatorExpr)); !c.Known || c.Holds || c.Found != "String" {
		t.Errorf("cast2 to Integer: %+v, want known, not holding, found String", c)
	}
}
