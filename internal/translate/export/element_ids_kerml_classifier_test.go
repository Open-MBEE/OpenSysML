package export

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// kermlClassifierModel declares a classifier X specializing A with keyword; the association
// keywords and `interaction` take two ends, a `type` the specialization its grammar requires.
func kermlClassifierModel(keyword string) string {
	body := ";"
	if keyword == "type" {
		return "package P {\n\tclass C;\n\ttype A specializes C;\n\ttype X specializes A;\n}\n"
	}
	if strings.HasPrefix(keyword, "assoc") || keyword == "interaction" {
		body = " { end feature a : C; end feature b : C; }"
	}
	return "package P {\n\tclass C;\n\t" + keyword + " A" + body + "\n\t" + keyword + " X specializes A" + body + "\n}\n"
}

// For every KerML classifier keyword the writer types `specializes` as a Subclassification
// at _sc0, and the identity reader states that same id for the clause.
func TestElementIDReaderReadsAKerMLClassifiersSubclassification(t *testing.T) {
	for _, keyword := range ast.KerMLTypeKeywords() {
		metaclass := ast.KerMLTypeMetaclass(keyword)
		relationship, id := mSubclassification, "P__X_sc0"
		if keyword == "type" {
			// A `type` is a Type, not a Classifier: its specializes is a plain Specialization.
			relationship, id = mSpecialization, "P__X_sp0"
		}
		t.Run(keyword, func(t *testing.T) {
			sf, _, doc := readerIndex(t, "k.kerml", kermlClassifierModel(keyword))
			docs := []ModelDocument{{File: sf, Root: doc.Root}}
			graph, _, encoders, err := modelToRDF(docs, IDQualifiedName)
			if err != nil {
				t.Fatalf("modelToRDF: %v", err)
			}
			if got := rdf.LocalName(graph.Type(rdf.ElementIRIForID("P__X"))); got != metaclass {
				t.Errorf("`%s X` is written as a %s, want %s", keyword, got, metaclass)
			}
			if got := rdf.LocalName(graph.Type(rdf.ElementIRIForID(id))); got != relationship {
				t.Errorf("the specializes of `%s X` is written at %s as %q, want %s", keyword, id, got, relationship)
			}
			minted, err := NewElementIDs(docs)
			if err != nil {
				t.Fatalf("NewElementIDs: %v", err)
			}
			if n := compareWrittenRelationships(t, "k.kerml", graph, encoders, minted); n == 0 {
				t.Fatalf("no relationship compared")
			}
			reader := NewElementIDReader([]ElementIDDocument{{File: sf, Root: doc.Root}})
			for _, e := range encoders {
				for node, fqn := range e.names.FQN {
					if fqn != "P::X" {
						continue
					}
					for _, rel := range headClauses(node) {
						if got, err := reader.ReadRelationship("k.kerml", rel); err != nil || got != id {
							t.Errorf("the specializes clause of `%s X` reads %q (%v), want %s", keyword, got, err, id)
						}
					}
				}
			}
		})
	}
}
