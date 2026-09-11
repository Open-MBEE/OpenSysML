package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
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
