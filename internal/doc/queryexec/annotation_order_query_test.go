package queryexec

import (
	"testing"
)

const annotationOrderFixture = `
private import DocumentQueries::*;
private import ScalarValues::*;
metadata def Outer {
	attribute nested {
		attribute value : String;
	}
}
metadata def Tag { attribute label : String; }
part a {
	@Outer { nested { value = "first"; } }
	@Tag { label = "second"; }
}
calc def Q :> Query {
	in root : Element;
	Project(
		source = Named(qualifiedName = "Observatory::a"),
		properties = ("name", "annotations")
	)
}
`

// A nested binding of an earlier annotation precedes a direct binding of a
// later one: the bulk read emits each annotation's values in declaration order.
func TestQueryOrdersNestedAndDirectAnnotationBindings(t *testing.T) {
	fixture := loadExecutionFixture(t, annotationOrderFixture)
	result, err := Execute(fixture.program(t, "Q"),
		Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model}, Bindings{
			"root": {ElementValue(fixture.symbol(t, "a"))},
		}, Options{})
	if err != nil {
		t.Fatalf("execute Q: %v", err)
	}
	got := cellStrings(t, result, "annotations")
	want := [][]string{{"Observatory::Outer::nested.value=first", "Observatory::Tag::label=second"}}
	if len(got) != len(want) {
		t.Fatalf("annotations = %v, want %v", got, want)
	}
	for i := range want {
		if len(got[i]) != len(want[i]) {
			t.Fatalf("annotations = %v, want %v", got, want)
		}
		for j := range want[i] {
			if got[i][j] != want[i][j] {
				t.Fatalf("annotations = %v, want %v", got, want)
			}
		}
	}
}
