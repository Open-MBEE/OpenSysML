package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// `all T` is typed T[0..*]: every element is an instance of T — the type a classifier is, the
// types a feature is typed by — and the extent holds any number of them, as BaseFunctions::'all'
// declares. A name resolving to nothing leaves only what the library function declares.
func TestExtentExpressionTypes(t *testing.T) {
	m, s := collectionModel(t, `
		variation part choice : C { variant part small; variant part large; }
		attribute everyC = all C;
		attribute everyChoice = all choice;
		attribute everyString = all String;
		attribute nothing = all Missing;`)
	wantValueTypes(t, m, s, "everyC", "C")
	wantValueTypes(t, m, s, "everyChoice", "C")
	wantValueTypes(t, m, s, "everyString", "String")
	wantValueTypes(t, m, s, "nothing", "Anything")
	for _, name := range []string{"everyC", "everyChoice", "everyString", "nothing"} {
		if r, ok := m.CollectionValues(s, valueOf(t, s, name)); !ok || r.Text() != "[0..*]" {
			t.Errorf("%s: %s %v, want [0..*]", name, r.Text(), ok)
		}
	}
	for _, name := range []string{"everyC", "everyChoice", "everyString"} {
		if elements, ok := m.CollectionElements(s, valueOf(t, s, name)); !ok || len(elements) != 1 {
			t.Errorf("%s: elements %v, want one", name, elements)
		}
	}
	if _, ok := m.CollectionElements(s, valueOf(t, s, "nothing")); ok {
		t.Error("nothing: elements typed though its type resolves to nothing")
	}
	if ExtentTypeName(valueOf(t, s, "everyC")) == nil || ExtentTypeName(&ast.LiteralInteger{}) != nil {
		t.Error("ExtentTypeName: names the operand of `all` alone")
	}
}
