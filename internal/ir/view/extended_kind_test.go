package view

import (
	"strings"
	"testing"
)

// A declaration's user-defined keywords head its kind after its modifiers, and
// the kind classifiers read through them to the keyword.
func TestExtendedKindKeepsTheDeclarationOrder(t *testing.T) {
	out, err := render(t, "spec-compartments.sysml", "HeadingViews::nodes").Write(FormText)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"abstract #command part def Launcher", "#command requirement def Goal", "#command verification case def Audit"} {
		if !strings.Contains(out, want) {
			t.Errorf("text rendering lacks %q:\n%s", want, out)
		}
	}
	for kind, classify := range map[string]func(string) bool{
		"#command requirement def":       requirementNodeKind,
		"abstract #command concern":      requirementNodeKind,
		"#command verification case def": caseNodeKind,
		"#safety #command use case":      caseNodeKind,
	} {
		if !classify(kind) {
			t.Errorf("%q is not classified by its keyword", kind)
		}
	}
	parent, subject := &Node{Kind: "#command requirement def"}, &Node{Kind: "subject"}
	if !parameterSymbol(parent, subject) {
		t.Error("a prefixed requirement's subject is not a parameter symbol")
	}
}
