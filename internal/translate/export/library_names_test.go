package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A graph is a version of a bundled library document only when every root is
// one of the top-level packages that document declares; a graph rooted at a
// nested library element, even under its normative id and name, is not the
// document and must not displace it.
func TestLibraryDocumentRequiresTopLevelPackageRoots(t *testing.T) {
	catalog := identity.LibraryCatalog(libs.NewModelIndex())
	byFQN := make(map[string]*identity.LibraryElement)
	for _, el := range catalog.Elements() {
		byFQN[el.FQN] = el
	}
	root := func(fqn, metaclass string) *element {
		el, ok := byFQN[fqn]
		if !ok {
			t.Fatalf("%s is not catalogued", fqn)
		}
		return &element{metaclass: metaclass, qname: fqn, elementID: el.ID}
	}
	const scalarValues = "Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml"
	cases := []struct {
		name  string
		roots []*element
		want  string
	}{
		{"top-level package", []*element{root("ScalarValues", "Package")}, scalarValues},
		{"nested element", []*element{root("ScalarValues::Real", "DataType")}, ""},
		{"nested element written as a package", []*element{root("ScalarValues::Real", "Package")}, ""},
		{"package beside a nested element", []*element{root("ScalarValues", "Package"), root("ScalarValues::Real", "DataType")}, ""},
		{"packages of two documents", []*element{root("ScalarValues", "Package"), root("Base", "Package")}, ""},
		{"user package under a library id", []*element{{metaclass: "Package", qname: "Mine", elementID: byFQN["ScalarValues"].ID}}, ""},
	}
	for _, tc := range cases {
		if got := libraryDocument(tc.roots); got != tc.want {
			t.Errorf("%s: libraryDocument = %q, want %q", tc.name, got, tc.want)
		}
	}
}
