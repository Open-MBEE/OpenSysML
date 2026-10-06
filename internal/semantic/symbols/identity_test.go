package symbols

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Two symbols built from one declaration in one document are one element,
// whichever scope tree built them.
func TestSameElementAcrossScopeTrees(t *testing.T) {
	span := source.Span{Offset: 10, Len: 10}
	a := &Symbol{Name: "Rover", DocName: "fleet.sysml", DeclSpan: span}
	b := &Symbol{Name: "Rover", DocName: "fleet.sysml", DeclSpan: span}
	if !SameElement(a, b) || KeyOf(a) != KeyOf(b) {
		t.Errorf("symbols of one declaration are not the same element")
	}
}

func TestSameElementDistinguishesDeclarations(t *testing.T) {
	doc := &Symbol{Name: "Rover", DocName: "fleet.sysml", DeclSpan: source.Span{Offset: 10, Len: 10}}
	for name, other := range map[string]*Symbol{
		"another span":     {Name: "Rover", DocName: "fleet.sysml", DeclSpan: source.Span{Offset: 30, Len: 10}},
		"another document": {Name: "Rover", DocName: "rover.sysml", DeclSpan: source.Span{Offset: 10, Len: 10}},
	} {
		if SameElement(doc, other) {
			t.Errorf("%s is the same element", name)
		}
	}
}

// A symbol with no declaring document — a library symbol restored from cache —
// is only itself, since nothing else identifies it.
func TestSameElementOfACachedSymbolIsItself(t *testing.T) {
	cached := &Symbol{Name: "Base"}
	if !SameElement(cached, cached) {
		t.Error("a cached symbol is not itself")
	}
	if SameElement(cached, &Symbol{Name: "Base"}) {
		t.Error("two cached symbols of one name are the same element")
	}
}

// A nil symbol declares nothing, so it is no element, itself included.
func TestSameElementOfNil(t *testing.T) {
	sym := &Symbol{Name: "Rover", DocName: "fleet.sysml"}
	if SameElement(nil, nil) || SameElement(sym, nil) || SameElement(nil, sym) {
		t.Error("a nil symbol was taken for an element")
	}
}

// Each relationship written on an element keys as its own element: distinct
// from its owner, its siblings, and stable across Models.
func TestKeyOfImplicitRelationships(t *testing.T) {
	owner := &Symbol{Name: "p", DocName: "pkg.sysml", DeclSpan: source.Span{Offset: 5, Len: 10}}
	first := &Symbol{Kind: SymbolRelationship, DocName: "pkg.sysml",
		DeclSpan: source.Span{Offset: 20, Len: 3},
		Implicit: &ImplicitRelationship{Owner: owner, Ordinal: 0}}
	second := &Symbol{Kind: SymbolRelationship, DocName: "pkg.sysml",
		DeclSpan: source.Span{Offset: 30, Len: 4},
		Implicit: &ImplicitRelationship{Owner: owner, Ordinal: 1}}
	firstAgain := &Symbol{Kind: SymbolRelationship, DocName: "pkg.sysml",
		DeclSpan: source.Span{Offset: 20, Len: 3},
		Implicit: &ImplicitRelationship{Owner: owner, Ordinal: 0}}
	if !SameElement(first, firstAgain) || KeyOf(first) != KeyOf(firstAgain) {
		t.Error("one written relationship rebuilt is not the same element")
	}
	if SameElement(first, second) || KeyOf(first) == KeyOf(second) {
		t.Error("two written relationships of one owner are one element")
	}
	if SameElement(first, owner) || KeyOf(first) == KeyOf(owner) {
		t.Error("a written relationship is its owner")
	}
	if KeyOf(first).String() == KeyOf(owner).String() {
		t.Error("key strings collide for a relationship and its owner")
	}
}
