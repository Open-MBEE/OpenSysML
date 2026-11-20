package queryexec

import (
	"slices"
	"testing"
)

// An alias writes its Membership's memberName and memberShortName (KerML 8.3.2.4.3):
// the Membership itself declares no name, as the API-JSON writer states none.
func TestExecuteAliasMembershipDeclaresNoName(t *testing.T) {
	fixture := loadExecutionFixture(t, `
package Model {
	part def D;
	part def E;
	protected alias <s> m for E;
	alias n for D;
}
calc def Q :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, relationships = true), type = "Membership", exact = true),
		properties = ("memberName", "memberShortName", "declaredName", "declaredShortName"),
		columns = (
			Column(name = "declared", expression = Element::declaredName ?? "none"),
			Column(name = "declaredShort", expression = Element::declaredShortName ?? "none")
		)
	)
}
calc def Named :> Query {
	in root : Element;
	Project(
		source = WhereFeature(
			source = Descendants(source = root, relationships = true),
			feature = "declaredName", operator = "=", value = "m"
		),
		properties = ("memberName")
	)
}`)
	root := Bindings{"root": {ElementValue(fixture.symbol(t, "Model"))}}
	result, err := fixture.execute(t, "Q", root, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	for column, want := range map[string][][]string{
		"memberName":        {{"m"}, {"n"}},
		"memberShortName":   {{"s"}, nil},
		"declaredName":      {nil, nil},
		"declaredShortName": {nil, nil},
		"declared":          {{"none"}, {"none"}},
		"declaredShort":     {{"none"}, {"none"}},
	} {
		if got := cellStrings(t, result, column); !slices.EqualFunc(got, want, slices.Equal) {
			t.Errorf("%s cells = %q, want %q", column, got, want)
		}
	}
	named, err := fixture.execute(t, "Named", root, Options{})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := cellStrings(t, named, "memberName"); len(got) != 0 {
		t.Errorf("elements declaring the name m = %q, want none", got)
	}
}
