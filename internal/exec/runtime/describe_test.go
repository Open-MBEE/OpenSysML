package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
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
	ctx := NewContext(typedModel(model, resolver), 1000)
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

// The article follows how a kind is said, not how it is spelled: "use case" is
// written with a vowel and said with a consonant.
func TestArticleForEveryKindWord(t *testing.T) {
	// Every word a kind begins with that takes "an"; the rest take "a".
	an := map[string]bool{
		"abstract": true, "accept": true, "action": true, "actor": true, "alias": true,
		"allocation": true, "analysis": true, "assert": true, "assignment": true, "assoc": true,
		"assume": true, "attribute": true, "entry": true, "enum": true, "exit": true,
		"expr": true, "expression": true, "if": true, "include": true, "individual": true,
		"interaction": true, "interface": true, "item": true, "objective": true,
		"occurrence": true, "unknown": true,
	}
	words := map[string]bool{}
	for kind := ast.DefinitionKind(0); kind.String() != "unknown"; kind++ {
		words[kind.String()] = true
	}
	for kind := ast.UsageKind(0); kind.String() != "unknown"; kind++ {
		words[kind.String()] = true
	}
	// A symbol without a written declaration is named by its classification.
	for kind := symbols.SymbolUnknown; kind == symbols.SymbolUnknown || kind.Named(); kind++ {
		words[(&symbols.Symbol{Kind: kind}).Notation()] = true
	}
	// The keywords a kind may be written behind, and the kinds callers name.
	for _, word := range []string{
		"abstract part", "variation part", "variant part", "assert constraint", "assume constraint",
		"require constraint", "var feature", "perform action", "include use case", "succession flow",
		"entry action", "do action", "exit action", "datatype", "feature", "#meta part",
		"constraint", "requirement", "analysis case", "",
	} {
		words[word] = true
	}
	if !words["use case"] || !words["use case def"] || !words["attribute def"] {
		t.Fatalf("the kind words are not all enumerated: %v", words)
	}
	for word := range words {
		first, _, _ := strings.Cut(word, " ")
		want := "a"
		if an[first] {
			want = "an"
		}
		if got := diag.IndefiniteArticle(word); got != want {
			t.Errorf("diag.IndefiniteArticle(%q) = %q, want %q", word, got, want)
		}
	}
}

// The messages that name a use case read "a use case", through each caller.
func TestKindErrorsNameAUseCase(t *testing.T) {
	_, _, root := parseAndBuildModel(t, `
use case def Drive;
use case drive : Drive;
attribute def Mass;
`)
	def, usage := resolveSymbol(t, root, "Drive"), resolveSymbol(t, root, "drive")
	for got, want := range map[string]string{
		describeDecl(def.Decl):   "a use case def",
		describeDecl(usage.Decl): "a use case usage",
		notOfKind(ErrNotAnAnalysis, def, "analysis case").Error(): "not an analysis case: Drive is a use case def, " +
			"not an analysis case definition or usage",
		notOfKind(ErrNotAConstraint, usage, "constraint").Error(): "not a constraint: drive is a use case usage, " +
			"not a constraint definition or usage",
		notAnObject(def).Error(): "not an object: Drive is a use case def, which has no object to validate",
		notAnObject(resolveSymbol(t, root, "Mass")).Error(): "not an object: Mass is an attribute def, " +
			"which has no object to validate",
	} {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}
