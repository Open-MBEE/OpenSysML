package grpc

import (
	"slices"
	"testing"
)

// A filtered import, a library import and a filter reading a metaclass attribute answer
// importedMembership over the symbol structure as they do over the written graph.
func TestElementPropertiesReadFilteredAndLibraryImports(t *testing.T) {
	r := parseForElementProperties(t, `package P {
    metadata def Safety;
    part def A { @Safety; }
    abstract part def B;
    package ByMetadata { private import P::*[@Safety]; }
    package ByAttribute { private import P::*; filter SysML::Type::isAbstract; }
    package Library { private import ScalarValues::Real; }
}`)
	if got := propertyElementIDs(r.property("P__ByMetadata", "importedMembership").Values); !slices.Equal(got, []string{"P__A_om"}) {
		t.Errorf("ByMetadata imports %v, want the one annotated member P__A_om", got)
	}
	if got := propertyElementIDs(r.property("P__ByAttribute", "importedMembership").Values); !slices.Equal(got, []string{"P__B_om"}) {
		t.Errorf("ByAttribute imports %v, want the one abstract member P__B_om", got)
	}
	if got := r.property("P__Library", "importedMembership").Values; len(got) != 1 {
		t.Errorf("Library imports %d memberships, want the library's one for Real", len(got))
	}
}

// A library alias an import brings in is its Membership, read at the id the library's form writes.
func TestElementPropertiesIdentifyImportedLibraryAliases(t *testing.T) {
	r := parseForElementProperties(t, `package P { private import SI::*; }`)
	for i, value := range r.property("P", "importedMembership").Values {
		if value.GetElement().GetId() == "" {
			t.Fatalf("imported membership %d (%s) has no id", i, value.GetElement().GetMetaclass())
		}
	}
}

// Two filtered imports of one namespace each import their own filter package, so the
// namespace imports what either filter keeps.
func TestElementPropertiesReadEachFilteredImportOfANamespace(t *testing.T) {
	r := parseForElementProperties(t, `package P {
    metadata def Safety;
    metadata def Security;
    part def A { @Safety; }
    part def B { @Security; }
    package Both { private import P::*[@Safety]; private import P::*[@Security]; }
}`)
	if got := propertyElementIDs(r.property("P__Both", "importedMembership").Values); !slices.Equal(got, []string{"P__A_om", "P__B_om"}) {
		t.Errorf("Both imports %v, want P__A_om and P__B_om", got)
	}
	var packages []string
	for _, imp := range propertyElementIDs(r.property("P__Both", "ownedImport").Values) {
		packages = append(packages, propertyElementIDs(r.property(imp, "importedNamespace").Values)...)
	}
	if len(packages) != 2 || packages[0] == packages[1] {
		t.Fatalf("the two imports import %v, want a filter package each", packages)
	}
	for i, want := range []string{"P__A_om", "P__B_om"} {
		if got := propertyElementIDs(r.property(packages[i], "importedMembership").Values); !slices.Equal(got, []string{want}) {
			t.Errorf("filter package %s imports %v, want %s", packages[i], got, want)
		}
	}
}

// A filter package's condition is owned by the package through its filter membership, so the
// condition's parameters have no qualified name: the package is unnamed.
func TestElementPropertiesFilterPackageOwnsItsCondition(t *testing.T) {
	r := parseForElementProperties(t, `package P { metadata def M; package L { part a; } view v { expose L::**[@M]; } }`)
	for _, tc := range []struct{ element, property, want string }{
		{"P__v___400_pfilter", "owner", "Package"},
		{"P__v___400_pfilter", "owningNamespace", "Package"},
		{"P__v___400_pfilter", "owningRelationship", "ElementFilterMembership"},
	} {
		values := r.property(tc.element, tc.property).Values
		if len(values) != 1 || values[0].GetElement().Metaclass != tc.want {
			t.Errorf("%s.%s = %v, want one %s", tc.element, tc.property, values, tc.want)
		}
	}
	if values := r.property("P__v___400_pfilter_pin0", "qualifiedName").Values; len(values) != 0 {
		t.Errorf("the condition's parameter has the qualified name %v, want none", values)
	}
}
