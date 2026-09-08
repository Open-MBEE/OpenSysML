package semantics

import (
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
		part def C { attribute mass :> ISQ::mass; attribute name : String; }
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
// sequence is typed by what every element conforms to — nothing, so Anything, where
// the elements share no type.
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
	wantValueTypes(t, m, s, "mixed", "Anything")
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

// A body whose result the model cannot type — its parameter declares no type, so
// the member is unresolved — leaves the result the library's Anything, not a guess.
func TestCollectionUntypedBodyFallsBackToLibraryResult(t *testing.T) {
	m, s := collectionModel(t, `
		attribute unknown = cs->collect { in x; x.mass };
		attribute unknown2 = cs.{ in x; x.mass };
		attribute body = cs->collect { in x : C; { in y; y } };`)
	wantValueTypes(t, m, s, "unknown", "Anything")
	wantValueTypes(t, m, s, "unknown2", "Anything")
	wantValueTypes(t, m, s, "body", "Evaluation")
}

// A body that names the feature it values leads the typer back to itself; typing terminates.
func TestCollectionSelfReferentialBodyTerminates(t *testing.T) {
	m, s := collectionModel(t, `
		attribute total :> ISQ::mass = cs->collect { in x : C; total }->reduce '+';
		attribute loop = cs->collect { in x : C; loop };
		attribute loop2 = cs.{ in x : C; loop2 };
		attribute kept = cs->select { in x : C; kept == x };`)
	wantValueTypes(t, m, s, "total", "ScalarValue")
	wantValueTypes(t, m, s, "loop", "Anything")
	wantValueTypes(t, m, s, "loop2", "Anything")
	wantValueTypes(t, m, s, "kept", "C")
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
		attribute unknown = cs.{ in x; x.mass };
		attribute pairs = cs->collect { in x : C; (x.mass, x.mass) };
		attribute mixed = cs.{ in x : C; (true, 1) };
		attribute partly = cs.{ in x; (x, 1) };
		attribute partly2 = cs->collect { in x; (x, 1) };`)
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
