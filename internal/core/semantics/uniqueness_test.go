package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// libraryModelWithDoc is stdlibModelWithDoc with the standard library marked
// as bundled library content, as the workspace loads it.
func libraryModelWithDoc(t *testing.T, name, src string) (*Model, *symbols.Scope) {
	t.Helper()
	p := parser.New(source.New(name, []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := stdlibIndex(t)
	for _, doc := range idx.Documents() {
		idx.MarkLibrary(doc)
	}
	idx.AddDocument(name, root)
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	r.ResolveDocument(name, root)
	return m, idx.DocumentRoot(name)
}

// TestIsUniqueDeclarationAndDefault covers KerML 7.3.4.4: a feature is unique
// unless it says nonunique, whatever else it says.
func TestIsUniqueDeclarationAndDefault(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def A;
		part def H {
			part fresh : A[*];
			part orderedFresh : A[*] ordered;
			part repeats : A[*] nonunique;
			part orderedRepeats : A[*] ordered nonunique;
			part single : A;
		}
	}`)
	h := nested(t, sym(t, root, "P").Scope, "H")
	for name, want := range map[string]bool{
		"fresh": true, "orderedFresh": true, "repeats": false, "orderedRepeats": false, "single": true,
	} {
		if got := m.IsUnique(nested(t, h.Scope, name)); got != want {
			t.Errorf("IsUnique(%s) = %v, want %v", name, got, want)
		}
	}
	if !m.IsUnique(nil) {
		t.Errorf("IsUnique(nil) = false, want true")
	}
}

// TestIsUniqueInheritsThroughRedefinition: an unstated redefinition is unique only
// when every redefined feature is (KerML 7.3.4.5); subsetting starts afresh.
func TestIsUniqueInheritsThroughRedefinition(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def A;
		part def Base {
			part us : A[*];
			part rs : A[*] nonunique;
		}
		part def Mixed { part ms : A[*] nonunique; }
		part def Inherits :> Base {
			part :>> us;
			part :>> rs;
			part restated :>> rs = ();
			part sub :> rs;
			part tightened :>> rs = ();
		}
		part def Deep :> Inherits { part :>> rs; }
		part def Both :> Base, Mixed { part both :>> us, ms; }
		part def Either :> Base, Mixed { part either :>> us; }
	}`)
	p := sym(t, root, "P").Scope
	inherits := nested(t, p, "Inherits")
	cases := []struct {
		owner *symbols.Symbol
		name  string
		want  bool
	}{
		{inherits, "us", true},
		{inherits, "rs", false},
		{inherits, "restated", false},
		{inherits, "sub", true},
		{nested(t, p, "Deep"), "rs", false},
		{nested(t, p, "Both"), "both", false},
		{nested(t, p, "Either"), "either", true},
	}
	for _, tc := range cases {
		if got := m.IsUnique(nested(t, tc.owner.Scope, tc.name)); got != tc.want {
			t.Errorf("IsUnique(%s::%s) = %v, want %v", tc.owner.Name, tc.name, got, tc.want)
		}
	}
	// A false answer is cached as false, not dropped as a zero value.
	rs := nested(t, inherits.Scope, "rs")
	if cached, ok := m.unique[rs]; !ok || cached {
		t.Errorf("cache for Inherits::rs = (%v, %v), want (false, true)", cached, ok)
	}
	if m.IsUnique(rs) {
		t.Errorf("IsUnique(Inherits::rs) = true on the cached path, want false")
	}
}

// TestUniquenessConformanceUsesEffectiveUniqueness: a nonunique redefinition is
// judged against what its target effectively is, not what it locally says.
func TestUniquenessConformanceUsesEffectiveUniqueness(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def A;
		part def Base {
			part us : A[*];
			part rs : A[*] nonunique;
		}
		part def Middle :> Base {
			part :>> us;
			part :>> rs;
		}
		part def Leaf :> Middle {
			part :>> us [*] nonunique;
			part :>> rs [*] nonunique;
		}
	}`)
	p := sym(t, root, "P").Scope
	leaf := nested(t, p, "Leaf")
	middle := nested(t, p, "Middle")
	rs := nested(t, leaf.Scope, "rs")
	if got := m.ConformanceViolations(rs); len(got) != 0 {
		t.Errorf("ConformanceViolations(Leaf::rs) = %+v, want none: Middle::rs inherits nonunique", got)
	}
	us := nested(t, leaf.Scope, "us")
	got := m.ConformanceViolations(us)
	if len(got) != 1 || got[0].Kind != ViolationUniqueness || got[0].Target != nested(t, middle.Scope, "us") {
		t.Errorf("ConformanceViolations(Leaf::us) = %+v, want one uniqueness violation against Middle::us", got)
	}
}

// TestIsUniqueInheritsIntoMetadataBody: a metadata body declaration redefines the
// metadata type's feature of its name (KerML 7.4.7), and so takes its uniqueness.
func TestIsUniqueInheritsIntoMetadataBody(t *testing.T) {
	m, root := buildModel(t, `package P {
		metadata def M {
			attribute repeats : ScalarValues::Real[0..*] ordered nonunique;
			attribute fresh : ScalarValues::Real[0..*] ordered;
		}
		part def A {
			metadata inline : M { repeats = (1, 1); fresh = (1, 2); }
		}
		metadata elsewhere : M about A { repeats = (2, 2); fresh = (3, 4); }
	}`)
	p := sym(t, root, "P").Scope
	for _, usage := range []*symbols.Symbol{nested(t, p, "A", "inline"), nested(t, p, "elsewhere")} {
		if m.IsUnique(nested(t, usage.Scope, "repeats")) {
			t.Errorf("IsUnique(%s::repeats) = true, want false through M::repeats", usage.Name)
		}
		if !m.IsUnique(nested(t, usage.Scope, "fresh")) {
			t.Errorf("IsUnique(%s::fresh) = false, want true through M::fresh", usage.Name)
		}
	}
}

// TestIsUniqueTerminatesOnRedefinitionCycle covers that a redefinition chain
// closing on itself contributes nothing and does not recurse forever. The
// chain runs through a specialization cycle, since a redefinition names an
// inherited feature and never a sibling (KerML 8.2.3.5.2).
func TestIsUniqueTerminatesOnRedefinitionCycle(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def A :> B {
			part x :>> y;
			part z :>> w;
		}
		part def B :> A {
			part y :>> x;
			part w :>> z nonunique;
		}
	}`)
	p := sym(t, root, "P").Scope
	a, b := nested(t, p, "A"), nested(t, p, "B")
	if !m.IsUnique(nested(t, a.Scope, "x")) || !m.IsUnique(nested(t, b.Scope, "y")) {
		t.Errorf("a cycle of unstated redefinitions should stay unique by default")
	}
	if m.IsUnique(nested(t, a.Scope, "z")) {
		t.Errorf("IsUnique(z) = true, want false through the nonunique w it redefines")
	}
	if m.IsUnique(nested(t, b.Scope, "w")) {
		t.Errorf("IsUnique(w) = true, want false as declared")
	}
}

// TestIsUniqueOnUnresolvedSiblingRedefinitions covers that redefinitions naming
// siblings of the redefining feature stay unresolved, so each feature keeps its
// own declared uniqueness.
func TestIsUniqueOnUnresolvedSiblingRedefinitions(t *testing.T) {
	m, root := buildModel(t, `package P {
		part def H {
			part x :>> y;
			part y :>> x;
			part z :>> w;
			part w :>> z nonunique;
		}
	}`)
	h := nested(t, sym(t, root, "P").Scope, "H")
	for _, name := range []string{"x", "y", "z"} {
		if !m.IsUnique(nested(t, h.Scope, name)) {
			t.Errorf("IsUnique(%s) = false, want true: its redefinition target is unresolved", name)
		}
	}
	if m.IsUnique(nested(t, h.Scope, "w")) {
		t.Errorf("IsUnique(w) = true, want false as declared")
	}
}

// TestIsUniqueLibraryCollections: Set, OrderedSet, Map and OrderedMap elements are
// unique by the library's note (docs/project/omg-issues.md); Bag and List are not.
func TestIsUniqueLibraryCollections(t *testing.T) {
	m, root := libraryModelWithDoc(t, "collections.sysml", `package P {
		private import ScalarValues::*;
		private import Collections::*;
		attribute bag : Bag { :>> elements = (1, 1); }
		attribute list : List { :>> elements = (1, 1); }
		attribute set : Set { :>> elements = (1, 2); }
		attribute orderedSet : OrderedSet { :>> elements = (1, 2); }
		attribute map : Map { :>> elements = (1, 2); }
		attribute orderedMap : OrderedMap { :>> elements = (1, 2); }
	}`)
	p := sym(t, root, "P").Scope
	for name, want := range map[string]bool{
		"bag": false, "list": false, "set": true, "orderedSet": true, "map": true, "orderedMap": true,
	} {
		elements := nested(t, p, name, "elements")
		if got := m.IsUnique(elements); got != want {
			t.Errorf("IsUnique(%s.elements) = %v, want %v", name, got, want)
		}
	}
	for _, fqn := range []string{
		"Collections::UniqueCollection::elements", "Collections::Map::elements",
		"Collections::OrderedSet::elements", "Collections::OrderedMap::elements",
	} {
		if lib := m.librarySymbol(fqn); lib == nil || !m.IsUnique(lib) {
			t.Errorf("IsUnique(%s) = false, want true by the library note", fqn)
		}
	}
	if lib := m.librarySymbol("Collections::Collection::elements"); lib == nil || m.IsUnique(lib) {
		t.Errorf("IsUnique(Collections::Collection::elements) = true, want false as declared")
	}
}
