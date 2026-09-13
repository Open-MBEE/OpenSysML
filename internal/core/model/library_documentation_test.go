package model

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
)

// libraryNotesModel renders the documentation of a standard-library element,
// which only the library's own source text can answer.
const libraryNotesModel = `package LibraryDocs {
	private import DocumentQueries::*;
	private import KerML::Root::Element;

	calc def Doc :> Query {
		in root : Element;
		Project(source = root, properties = ("name", "documentation"))
	}

	part def PartNotes :> Document {
		attribute redefines title = "Library notes";

		part notes : Definitions {
			attribute redefines term = "name";
			attribute redefines description = "documentation";
			calc rows : Doc {
				in root = Parts::parts;
			}
		}
	}
}
`

// A workspace document reads the documentation of standard-library elements
// from the library files its index was built from.
func TestRenderDocumentMarkdownReadsLibraryDocumentation(t *testing.T) {
	ws := openDoc(t, "notes.sysml", libraryNotesModel)
	for _, d := range ws.Diagnostics("notes.sysml") {
		if d.Severity == passes.SeverityError {
			t.Fatalf("model did not analyse cleanly: %v", d)
		}
	}
	markdown, err := ws.RenderDocumentMarkdown("LibraryDocs::PartNotes", docrender.MarkdownOptions{})
	if err != nil {
		t.Fatalf("RenderDocumentMarkdown: %v", err)
	}
	want := "**parts** — parts is the base feature of all part properties."
	if !strings.Contains(markdown, want) {
		t.Errorf("Markdown lacks the library element's documentation:\n%s", markdown)
	}
}
