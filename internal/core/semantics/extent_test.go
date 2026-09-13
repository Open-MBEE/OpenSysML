package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// `all T` is typed T[0..*]: every element is an instance of T — the type a classifier is, the
// types a feature is typed by — and the extent holds any number of them, as BaseFunctions::'all'
// declares. A name resolving to nothing, or to an element that is no type — a package has
// no extent — leaves only what the library function declares.
func TestExtentExpressionTypes(t *testing.T) {
	m, s := collectionModel(t, `
		variation part choice : C { variant part small; variant part large; }
		package Pkg;
		attribute everyC = all C;
		attribute everyChoice = all choice;
		attribute everyString = all String;
		attribute nothing = all Missing;
		attribute noType = all Pkg;`)
	wantValueTypes(t, m, s, "everyC", "C")
	wantValueTypes(t, m, s, "everyChoice", "C")
	wantValueTypes(t, m, s, "everyString", "String")
	wantValueTypes(t, m, s, "nothing", "Anything")
	wantValueTypes(t, m, s, "noType", "Anything")
	for _, name := range []string{"everyC", "everyChoice", "everyString", "nothing", "noType"} {
		if r, ok := m.CollectionValues(s, valueOf(t, s, name)); !ok || r.Text() != "[0..*]" {
			t.Errorf("%s: %s %v, want [0..*]", name, r.Text(), ok)
		}
	}
	for _, name := range []string{"everyC", "everyChoice", "everyString"} {
		if elements, ok := m.CollectionElements(s, valueOf(t, s, name)); !ok || len(elements) != 1 {
			t.Errorf("%s: elements %v, want one", name, elements)
		}
	}
	for _, name := range []string{"nothing", "noType"} {
		if _, ok := m.CollectionElements(s, valueOf(t, s, name)); ok {
			t.Errorf("%s: elements typed though its name resolves to no type", name)
		}
	}
	if _, ok := m.ExtentOperand(s, valueOf(t, s, "nothing")); ok {
		t.Error("nothing: an unresolved name resolved")
	}
	pkg, ok := m.ExtentOperand(s, valueOf(t, s, "noType"))
	if !ok || pkg == nil || pkg.Name != "Pkg" || IsType(pkg) {
		t.Errorf("noType: operand %v %v, want the package, which is no type", pkg, ok)
	}
	if m.ExtentType(s, valueOf(t, s, "noType")) != nil {
		t.Error("noType: a package answered as the extent's type")
	}
	if c := m.ExtentType(s, valueOf(t, s, "everyC")); c == nil || !IsType(c) {
		t.Errorf("everyC: type %v, want the classifier C", c)
	}
	if ExtentTypeName(valueOf(t, s, "everyC")) == nil || ExtentTypeName(&ast.LiteralInteger{}) != nil {
		t.Error("ExtentTypeName: names the operand of `all` alone")
	}
}

// An extent conforms to a type as every instance of its type does: `all C` holds no String,
// `all choice` holds values of the variation, C's; one of a name resolving to no type is unknown.
func TestExtentExpressionConformance(t *testing.T) {
	m, s := collectionModel(t, `
		variation part choice : C { variant part small; variant part large; }
		attribute everyC = all C;
		attribute everyChoice = all choice;
		attribute everyString = all String;
		attribute nothing = all Missing;`)
	types := map[string]*symbols.Symbol{
		"C":        sym(t, s.Parent(), "C"),
		"String":   m.libSymbol(fqnString),
		"Integer":  m.libSymbol(fqnInteger),
		"Anything": m.libSymbol(fqnAnything),
	}
	for _, tc := range []struct {
		name, want   string
		known, holds bool
	}{
		{"everyC", "C", true, true},
		{"everyC", "Anything", true, true},
		{"everyC", "String", true, false},
		{"everyChoice", "C", true, true},
		{"everyChoice", "String", true, false},
		{"everyString", "String", true, true},
		{"everyString", "Integer", true, false},
		{"nothing", "String", false, false},
	} {
		want := types[tc.want]
		if want == nil {
			t.Fatalf("%s: no such type", tc.want)
		}
		if got := m.ExprConformsTo(s, valueOf(t, s, tc.name), want); got.Known != tc.known || got.Holds != tc.holds {
			t.Errorf("%s conforms to %s: %+v, want known=%v holds=%v", tc.name, tc.want, got, tc.known, tc.holds)
		}
	}
}
