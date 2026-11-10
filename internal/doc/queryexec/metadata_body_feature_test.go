package queryexec

import (
	"slices"
	"testing"
)

// A metadata body's `mission = "Hubble"` on the query leg: a ReferenceUsage declaring no
// name (its implied Redefinition names it) that may time-vary under its MetadataUsage, an
// occurrence typed by the metadata definition — as reflection and the pilot answer it.
func TestMetadataBodyFeatureRowAnswersReflection(t *testing.T) {
	fixture := loadExecutionFixture(t, `
package M {
	part def T; part telescope : T;
	metadata def Heritage { attribute mission : String; }
	@Heritage about telescope { mission = "Hubble"; }
}
calc def Body :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, relationships = true), type = "ReferenceUsage", exact = true),
		properties = ("declaredName", "name", "isVariable")
	)
}
`)
	result, err := fixture.execute(t, "Body", Bindings{
		"root": {ElementValue(fixture.symbol(t, "M"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute Body: %v", err)
	}
	if n := len(result.Rows()); n != 1 {
		t.Fatalf("ReferenceUsage rows = %d, want the body feature alone", n)
	}
	for column, want := range map[string][]string{"declaredName": nil, "name": {"mission"}} {
		if got := cellStrings(t, result, column)[0]; !slices.Equal(got, want) {
			t.Errorf("mission %s = %q, want %q", column, got, want)
		}
	}
	if got := cellBooleans(t, result, "isVariable")[0]; !slices.Equal(got, []bool{true}) {
		t.Errorf("mission isVariable = %v, want [true]: its owning MetadataUsage is an occurrence", got)
	}
}
