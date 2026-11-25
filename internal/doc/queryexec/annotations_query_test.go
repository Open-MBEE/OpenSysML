package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const annotationsQueryFixture = `
package Meta {
	private import DocumentQueries::*;
	private import ScalarValues::*;
	metadata def Tag { attribute label : String; }
	part a;
	part b;
	part c;
	@Tag about a { label = "alpha"; }
	@Tag about b { label = "beta"; }
}
calc def AnnotationsQuery :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, relationships = true), type = "PartUsage", exact = true),
		properties = ("name", "annotations")
	)
}
calc def AnnotationsFilter :> Query {
	in root : Element;
	WhereFeature(
		source = Descendants(source = root, relationships = true),
		'feature' = "annotations", operator = "=", value = "Observatory::Meta::Tag::label=beta"
	)
}
`

// annotations reads the metadata of a whole row set in one query: each
// annotated row reports its binding and an unannotated row reports none.
func TestQueryReadsAnnotationsForARowSet(t *testing.T) {
	fixture := loadExecutionFixture(t, annotationsQueryFixture)
	result, err := Execute(fixture.program(t, "AnnotationsQuery"),
		Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model}, Bindings{
			"root": {ElementValue(fixture.symbol(t, "Meta"))},
		}, Options{})
	if err != nil {
		t.Fatalf("execute AnnotationsQuery: %v", err)
	}
	names := cellStrings(t, result, "name")
	annotations := cellStrings(t, result, "annotations")
	got := map[string][]string{}
	for i, name := range names {
		if len(name) != 1 {
			t.Fatalf("row %d name = %v", i, name)
		}
		got[name[0]] = annotations[i]
	}
	want := map[string][]string{
		"a": {"Observatory::Meta::Tag::label=alpha"},
		"b": {"Observatory::Meta::Tag::label=beta"},
		"c": nil,
	}
	for name, values := range want {
		found, ok := got[name]
		if !ok || len(found) != len(values) {
			t.Fatalf("annotations of %s = %q, want %q (all = %v)", name, found, values, got)
		}
		for i := range values {
			if found[i] != values[i] {
				t.Fatalf("annotations of %s = %q, want %q", name, found, values)
			}
		}
	}
}

const nestedAnnotationsQueryFixture = `
private import DocumentQueries::*;
private import ScalarValues::*;
metadata def Outer {
	attribute nested {
		attribute value : String;
	}
}
part a {
	@Outer { nested { value = "nested"; } }
}
part b;
calc def Q :> Query {
	in root : Element;
	Project(
		source = Union(
			source = Named(qualifiedName = "Observatory::a"),
			other = Named(qualifiedName = "Observatory::b")
		),
		properties = ("name", "annotations"), tolerate = true
	)
}
`

// A binding nested in an annotation body's own binding is part of the row's
// metadata: the bulk read reports its path and value without a symbol lookup.
func TestQueryReadsNestedAnnotationBindings(t *testing.T) {
	fixture := loadExecutionFixture(t, nestedAnnotationsQueryFixture)
	result, err := Execute(fixture.program(t, "Q"),
		Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model}, Bindings{
			"root": {ElementValue(fixture.symbol(t, "a"))},
		}, Options{})
	if err != nil {
		t.Fatalf("execute Q: %v", err)
	}
	names := cellStrings(t, result, "name")
	annotations := cellStrings(t, result, "annotations")
	got := map[string][]string{}
	for i, name := range names {
		if len(name) != 1 {
			t.Fatalf("row %d name = %v", i, name)
		}
		got[name[0]] = annotations[i]
	}
	want := map[string][]string{
		"a": {"Observatory::Outer::nested.value=nested"},
		"b": nil,
	}
	for name, values := range want {
		found := got[name]
		if len(found) != len(values) {
			t.Fatalf("annotations of %s = %q, want %q (all = %v)", name, found, values, got)
		}
		for i := range values {
			if found[i] != values[i] {
				t.Fatalf("annotations of %s = %q, want %q", name, found, values)
			}
		}
	}
}

// The same property filters a row set without a per-element read on the caller.
func TestQueryFiltersByAnnotationValue(t *testing.T) {
	fixture := loadExecutionFixture(t, annotationsQueryFixture)
	result, err := Execute(fixture.program(t, "AnnotationsFilter"),
		Context{Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model}, Bindings{
			"root": {ElementValue(fixture.symbol(t, "Meta"))},
		}, Options{})
	if err != nil {
		t.Fatalf("execute AnnotationsFilter: %v", err)
	}
	ids := map[string]bool{}
	for _, row := range result.Rows() {
		sym, _ := row.Element().Element()
		ids[symbols.FQNOf(sym)] = true
	}
	want := map[string]bool{"Observatory::Meta::b": true}
	if len(ids) != len(want) || !ids["Observatory::Meta::b"] {
		t.Fatalf("rows = %v, want Observatory::Meta::b alone", ids)
	}
}
