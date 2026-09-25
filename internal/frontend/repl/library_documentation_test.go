package repl

import "testing"

// A session reads the documentation of standard-library elements from the
// library files its index was built from, not only from what it was given.
func TestRunQueryReadsLibraryDocumentation(t *testing.T) {
	s := NewSession()
	res := s.Submit(`package LibraryDocs {
	private import DocumentQueries::*;
	private import KerML::Root::Element;

	calc def Doc :> Query {
		in root : Element;
		Project(source = root, properties = ("name", "documentation"))
	}
}`)
	if len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	got := run(t, s, "%run-query LibraryDocs::Doc root=Parts::Part")
	wants(t, got,
		"✓ Query LibraryDocs::Doc returned 1 row",
		"Row 1: Parts::Part",
		`name = "Part"`,
		`documentation = "Part is the most general class of objects that represent all or a part of a system.`,
	)
}
