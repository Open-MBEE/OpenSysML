package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// TestDescribeOperandEnumerationLiteral: an operator diagnostic names the
// literal an operand is, so the reader sees which enumeration it came from.
func TestDescribeOperandEnumerationLiteral(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
package D {
    enum def Color { red; green; }
}
attribute test = D::Color::red;
`)
	ctx := NewContext(NewModel(model, resolver), 1000)
	sym := resolveSymbol(t, root, "test")
	val, err := ctx.Eval(sym.Decl.(*ast.Usage).Value)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	if got := describeOperand(val); got != "the enumeration literal Color::red" {
		t.Errorf("describeOperand = %q", got)
	}
	// A literal that was never resolved is still described, never printed blank.
	if got := describeOperand(Value{Kind: ValEnumLiteral}); got != "the enumeration literal <unknown enumeration literal>" {
		t.Errorf("describeOperand of an unresolved literal = %q", got)
	}
}

// A diagnostic names a declaration as the notation writes it, since several
// spellings share one kind.
func TestDescribeDeclNamesTheWrittenDeclaration(t *testing.T) {
	_, _, root := parseAndBuildModel(t, `
datatype T;
part def Wheel;
part w : Wheel;
`)
	want := map[string]string{
		"T":     "a datatype usage",
		"Wheel": "a part def",
		"w":     "a part usage",
	}
	for name, description := range want {
		if got := describeDecl(resolveSymbol(t, root, name).Decl); got != description {
			t.Errorf("describeDecl(%s) = %q, want %q", name, got, description)
		}
	}
	if got := describeDecl(nil); got != "nothing" {
		t.Errorf("describeDecl(nil) = %q, want %q", got, "nothing")
	}
}
