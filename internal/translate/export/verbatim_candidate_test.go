package export

import (
	"os"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A written graph names the library members its types inherit and the candidate
// does not; that alone must not have the candidate encoded in the other id form too.
func TestLibraryReferencesDoNotEncodeTheCandidateTwice(t *testing.T) {
	notation := membershipModel(20)
	file := source.New("n.sysml", []byte(notation))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("fixture does not parse: %v", p.Diagnostics)
	}
	bare, err := encodeDocument(file, root, "", IDQualifiedName)
	if err != nil {
		t.Fatal(err)
	}
	written := graphOfNotation(t, notation)
	if written.Len() <= bare.graph.Len() {
		t.Fatalf("the written graph holds %d statements, the candidate's %d; want library references the candidate lacks",
			written.Len(), bare.graph.Len())
	}
	allocs := func(graph *rdf.Graph) float64 {
		return testing.AllocsPerRun(2, func() {
			out, err := ToSysML(graph)
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != notation {
				t.Fatalf("a graph of %d statements converts to\n%s\nwant the notation it was written from", graph.Len(), out)
			}
		})
	}
	without, with := allocs(bare.graph), allocs(written)
	if ratio := with / without; ratio > 1.5 {
		t.Errorf("converting the graph with its library references allocates %.0f, without them %.0f (%.2fx); want one candidate encoding",
			with, without, ratio)
	}
}

// A library document's root carries its normative uuid in either id form, so the
// candidate that agrees with a graph of qualified-name ids is the second one tried.
func TestLibraryDocumentKeepsItsTextInTheQualifiedNameForm(t *testing.T) {
	notation, err := os.ReadFile("../../workspace/libs/stdlib/Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml")
	if err != nil {
		t.Fatal(err)
	}
	for _, form := range []IDForm{IDQualifiedName, IDUUID} {
		file := source.New("ScalarValues.kerml", notation)
		p := parser.New(file)
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("the library document does not parse: %v", p.Diagnostics)
		}
		graph, err := ToRDFWith(file, root, form)
		if err != nil {
			t.Fatal(err)
		}
		out, err := ToSysML(graph)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != string(notation) {
			t.Errorf("id form %v converts to\n%s\nwant the document as written", form, out)
		}
	}
}
