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
