package queryexec

import (
	"slices"
	"testing"
)

// The ConjugatedPortDefinition a port definition implies declares no name: `~P` is the name its
// effectiveName() derives (SysML v2.1 8.3.12.2), as a projected column and as a reflective expression.
func TestConjugatedPortDefinitionRowDeclaresNoName(t *testing.T) {
	fixture := loadExecutionFixture(t, `
package C { port def P; }
calc def Conjugates :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, relationships = true), type = "ConjugatedPortDefinition", exact = true),
		properties = ("declaredName", "name", "declaredShortName"),
		columns = (
			Column(name = "declared", expression = Element::declaredName ?? "none"),
			Column(name = "effective", expression = Element::name ?? "none")
		)
	)
}
calc def NamedTilde :> Query {
	in root : Element;
	WhereFeature(source = Descendants(source = root, relationships = true), feature = "declaredName", operator = "=", value = "~P")
}
`)
	root := Bindings{"root": {ElementValue(fixture.symbol(t, "C"))}}
	result, err := fixture.execute(t, "Conjugates", root, Options{})
	if err != nil {
		t.Fatalf("execute Conjugates: %v", err)
	}
	if n := len(result.Rows()); n != 1 {
		t.Fatalf("ConjugatedPortDefinition rows = %d, want the conjugate of P alone", n)
	}
	for column, want := range map[string][]string{
		"declaredName": nil, "name": {"~P"}, "declaredShortName": nil,
		"declared": {"none"}, "effective": {"~P"},
	} {
		if got := cellStrings(t, result, column)[0]; !slices.Equal(got, want) {
			t.Errorf("~P %s = %q, want %q", column, got, want)
		}
	}
	filtered, err := fixture.execute(t, "NamedTilde", root, Options{})
	if err != nil {
		t.Fatalf("execute NamedTilde: %v", err)
	}
	if n := len(filtered.Rows()); n != 0 {
		t.Errorf("elements declaring the name ~P = %d, want none", n)
	}
}
