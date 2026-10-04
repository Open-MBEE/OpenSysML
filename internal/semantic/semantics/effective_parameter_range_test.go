package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// paramIn resolves the parameter named name in the scope of the behavior or
// type named owner at the document root.
func paramIn(t *testing.T, root *symbols.Scope, owner, name string) *symbols.Symbol {
	t.Helper()
	p, ok := sym(t, root, "test").Scope.LookupLocal(owner)
	if !ok {
		t.Fatalf("owner %q not found", owner)
	}
	q, ok := p.Scope.LookupLocal(name)
	if !ok {
		t.Fatalf("parameter %q of %q not found", name, owner)
	}
	return q
}

// TestEffectiveParameterRange walks §7.6.3: a parameter writing no multiplicity
// takes the nearest redefined or subsetted bound, the implicit [1..1] where it
// qualifies, else [0..*].
func TestEffectiveParameterRange(t *testing.T) {
	src := `
		package test {
			action def Seed { in energy : Energy[1..*]; }
			action def Bare { in x : Real; }
			action def One { in x : Real[1]; }
			action def Opt { in x : Real[0..1]; }
			action def Kwd { in attribute x : Real; }
			action def ItemParam { in item x; }
			action def FromSeed specializes Seed { in energy :>> energy; }
			action def FromBare specializes Bare { in x :>> x; }
			action def ReferenceSubset {
				part a [2];
				in part x ::> a;
			}
			part def Holder { part p : Part[1..*]; }
			part def Part { }
			part def Sub specializes Holder { part p :>> p; }
			calc def Dbl { in n : Real; inout r : Real; return : Real; }
		}`
	m, root := buildModelNamedWithKind(t, "<t>", source.KindSysML, src)

	cases := []struct {
		owner, param string
		want         Range
		optional     bool
	}{
		{"Bare", "x", UnboundedRange(), true}, // bare → [0..*]
		{"One", "x", AssumedRange(), false},   // [1] required
		{"Opt", "x", Range{Lower: Bound{Value: 0, Known: true}, Upper: Bound{Value: 1, Known: true}}, true},
		{"Kwd", "x", AssumedRange(), false}, // `in attribute` → implicit [1..1]
		{"ItemParam", "x", AssumedRange(), false},
		{"ReferenceSubset", "x", Range{Lower: Bound{Value: 2, Known: true}, Upper: Bound{Value: 2, Known: true}}, false},
		{"Seed", "energy", Range{Lower: Bound{Value: 1, Known: true}, Upper: Bound{Infinite: true, Known: true}}, false},
		{"FromSeed", "energy", Range{Lower: Bound{Value: 1, Known: true}, Upper: Bound{Infinite: true, Known: true}}, false}, // bare over [1..*] → required
		{"FromBare", "x", UnboundedRange(), true}, // bare over bare → optional
		{"Sub", "p", Range{Lower: Bound{Value: 1, Known: true}, Upper: Bound{Infinite: true, Known: true}}, false}, // redefines a [1..*] part
		{"Dbl", "n", UnboundedRange(), true}, // calc `in`
		{"Dbl", "r", UnboundedRange(), true}, // `inout`
	}
	for _, c := range cases {
		sym := paramIn(t, root, c.owner, c.param)
		if got := m.EffectiveParameterRange(sym); got != c.want {
			t.Errorf("EffectiveParameterRange(%s::%s) = %+v, want %+v", c.owner, c.param, got, c.want)
		}
		if got := m.OptionalParameter(sym); got != c.optional {
			t.Errorf("OptionalParameter(%s::%s) = %v, want %v", c.owner, c.param, got, c.optional)
		}
	}
}

// TestEffectiveParameterRangeSubsets: a parameter subsetting a bounded feature
// takes that bound, so bare over a [1..*] feature is required.
func TestEffectiveParameterRangeSubsets(t *testing.T) {
	src := `
		package test {
			part def Stock { part item : Item[1..*]; }
			part def Item { }
			action def Load { in item it subsets Stock::item : Item; }
		}`
	m, root := buildModelNamedWithKind(t, "<t>", source.KindSysML, src)
	sym := paramIn(t, root, "Load", "it")
	got := m.EffectiveParameterRange(sym)
	want := Range{Lower: Bound{Value: 1, Known: true}, Upper: Bound{Infinite: true, Known: true}}
	if got != want {
		t.Fatalf("EffectiveParameterRange(Load::it) = %+v, want %+v", got, want)
	}
	if m.OptionalParameter(sym) {
		t.Error("a parameter subsetting a [1..*] feature is required")
	}
}

// TestEffectiveParameterRangeIntersectsSubsets: the ranges a parameter's
// subsetted features give it intersect (KerML 1.0 §7.3.4.4): [0..1] and [1..*]
// bound the parameter to exactly one value, in either written order.
func TestEffectiveParameterRangeIntersectsSubsets(t *testing.T) {
	src := `
		package test {
			attribute f1 : Real[0..1];
			attribute f2 : Real[1..*];
			action def Both { in x : Real subsets f1, f2; }
			action def BothR { in x : Real subsets f2, f1; }
		}`
	m, root := buildModelNamedWithKind(t, "<t>", source.KindSysML, src)
	want := AssumedRange()
	for _, owner := range []string{"Both", "BothR"} {
		if got := m.EffectiveParameterRange(paramIn(t, root, owner, "x")); got != want {
			t.Errorf("EffectiveParameterRange(%s::x) = %+v, want %+v", owner, got, want)
		}
	}
}

// TestEffectiveParameterRangeIntersectsChainAndSubset: a chain element's subset
// targets intersect with the range the next chain element gives it: bare
// redefining an optional parameter while subsetting a [1..*] feature is required.
func TestEffectiveParameterRangeIntersectsChainAndSubset(t *testing.T) {
	src := `
		package test {
			attribute f : Real[1..*];
			calc def C { in a : Real[0..1]; }
			calc def D specializes C { in a :>> a subsets f; }
		}`
	m, root := buildModelNamedWithKind(t, "<t>", source.KindSysML, src)
	if got := m.EffectiveParameterRange(paramIn(t, root, "D", "a")); got != AssumedRange() {
		t.Errorf("EffectiveParameterRange(D::a) = %+v, want %+v", got, AssumedRange())
	}
}

// TestEffectiveParameterRangeSubsetCycle: parameters subsetting each other hold
// no range between them, so the walk terminates at the [0..*] default.
func TestEffectiveParameterRangeSubsetCycle(t *testing.T) {
	src := `
		package test {
			action def Cyc { in a : Real subsets b; in b : Real subsets a; }
		}`
	m, root := buildModelNamedWithKind(t, "<t>", source.KindSysML, src)
	for _, name := range []string{"a", "b"} {
		if got := m.EffectiveParameterRange(paramIn(t, root, "Cyc", name)); got != UnboundedRange() {
			t.Errorf("EffectiveParameterRange(Cyc::%s) = %+v, want %+v", name, got, UnboundedRange())
		}
	}
}
