package queryexec

import (
	"strings"
	"testing"
)

// TestExecuteWhereTypeExactKeepsOnlyTheNamedMetaclass: by default WhereType keeps
// every element conforming to the type; exact = true keeps those whose own
// metaclass is named, so a reflective walk mirrors each element once.
func TestExecuteWhereTypeExactKeepsOnlyTheNamedMetaclass(t *testing.T) {
	fixture := loadExecutionFixture(t, `
part def Heavy;
part root {
	part a : Heavy;
	item b;
	attribute c = 1.0;
	port d;
}
calc def Usages :> Query {
	in source : Element;
	WhereType(source = OwnedElements(source = source), type = "Usage")
}
calc def ExactUsages :> Query {
	in source : Element;
	WhereType(source = OwnedElements(source = source), type = "Usage", exact = true)
}
calc def ExactParts :> Query {
	in source : Element;
	WhereType(source = OwnedElements(source = source), type = ("PartUsage", "ItemUsage"), exact = true)
}
calc def ExactHeavy :> Query {
	in source : Element;
	WhereType(source = OwnedElements(source = source), type = "Heavy", exact = true)
}
calc def ExactQualified :> Query {
	in source : Element;
	WhereType(source = OwnedElements(source = source), type = "SysML::Systems::PartUsage", exact = true)
}
state def Machine {
	state first;
	state second;
	transition go first first then second;
}
calc def ExactTransitions :> Query {
	in source : Element;
	WhereType(source = Descendants(source = source), type = "TransitionUsage", exact = true)
}
calc def ExactStates :> Query {
	in source : Element;
	WhereType(source = Descendants(source = source), type = "StateUsage", exact = true)
}
`)
	bindings := Bindings{"source": {ElementValue(fixture.symbol(t, "root"))}}
	names := func(query string) string {
		result, err := fixture.execute(t, query, bindings, Options{})
		if err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		var out []string
		for _, row := range result.Rows() {
			sym, _ := row.Element().Element()
			out = append(out, sym.Name)
		}
		return strings.Join(out, " ")
	}
	if got := names("Usages"); got != "a b c d" {
		t.Fatalf("Usages = %q, want every usage", got)
	}
	if got := names("ExactUsages"); got != "" {
		t.Fatalf("ExactUsages = %q, want none: no element's own metaclass is Usage", got)
	}
	if got := names("ExactParts"); got != "a b" {
		t.Fatalf("ExactParts = %q, want the part and the item", got)
	}
	if got := names("ExactHeavy"); got != "" {
		t.Fatalf("ExactHeavy = %q, want none: a model type is not a metaclass", got)
	}
	if got := names("ExactQualified"); got != "a" {
		t.Fatalf("ExactQualified = %q, want the part: a qualified metaclass name is the metaclass", got)
	}
	machine := Bindings{"source": {ElementValue(fixture.symbol(t, "Machine"))}}
	exact := func(query string) string {
		result, err := fixture.execute(t, query, machine, Options{})
		if err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		var out []string
		for _, row := range result.Rows() {
			sym, _ := row.Element().Element()
			out = append(out, sym.Name)
		}
		return strings.Join(out, " ")
	}
	if got := exact("ExactTransitions"); got != "go" {
		t.Fatalf("ExactTransitions = %q, want the transition: its metaclass is TransitionUsage", got)
	}
	if got := exact("ExactStates"); got != "first second" {
		t.Fatalf("ExactStates = %q, want the two states and not the transition", got)
	}
}
