package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// Every KerML type keyword but `type` builds a Classifier, and each metaclass reads back to
// the usage kind the parser records.
func TestDeclaredClassifierMetaclasses(t *testing.T) {
	for keyword, metaclass := range kermlTypeMetaclass {
		if got, want := isClassifier(metaclass), keyword != "type"; got != want {
			t.Errorf("`%s` builds %s: classifier = %v, want %v", keyword, metaclass, got, want)
		}
		if _, ok := kermlTypeMetaclassUsage[metaclass]; !ok {
			t.Errorf("`%s` builds %s, which reads back to no usage kind", keyword, metaclass)
		}
	}
	for _, keyword := range []string{"datatype", "function"} {
		if !isClassifier(keywordMetaclass[keyword]) {
			t.Errorf("`%s` builds %s, which is not a Classifier", keyword, keywordMetaclass[keyword])
		}
	}
}

// kermlClassifierKeywords are the keywords that declare a KerML classifier, with the
// metaclass each builds (KerML 8.2.4 and 8.2.5).
var kermlClassifierKeywords = []struct{ keyword, metaclass string }{
	{"classifier", "Classifier"},
	{"class", "Class"},
	{"struct", "Structure"},
	{"assoc", "Association"},
	{"assoc struct", "AssociationStructure"},
	{"behavior", "Behavior"},
	{"predicate", "Predicate"},
	{"interaction", "Interaction"},
	{"datatype", "DataType"},
	{"function", "Function"},
	{"metaclass", "Metaclass"},
}

// TestKerMLClassifierSpecializesIsOneSubclassification: each classifier keyword is written
// with its KerML metaclass, and its `specializes` is a Subclassification under `_sc0` that
// the identity reader mints and reads under the same id.
func TestKerMLClassifierSpecializesIsOneSubclassification(t *testing.T) {
	for _, tc := range kermlClassifierKeywords {
		t.Run(tc.keyword, func(t *testing.T) {
			text := "package P {\n\t" + tc.keyword + " A;\n\t" + tc.keyword + " X specializes A;\n}\n"
			sf, _, doc := readerIndex(t, "k.kerml", text)
			docs := []ModelDocument{{File: sf, Root: doc.Root}}
			graph, _, encoders, err := modelToRDF(docs, IDQualifiedName)
			if err != nil {
				t.Fatalf("modelToRDF: %v", err)
			}
			if got := ontology.LocalName(graph.Type(rdf.ElementIRIForID("P__X"))); got != tc.metaclass {
				t.Errorf("P::X is written as a %s, want %s", got, tc.metaclass)
			}
			if got := ontology.LocalName(graph.Type(rdf.ElementIRIForID("P__X_sc0"))); got != mSubclassification {
				t.Errorf("P__X_sc0 is written as %q, want %s", got, mSubclassification)
			}
			if got := graph.Type(rdf.ElementIRIForID("P__X_sp0")); got != "" {
				t.Errorf("P__X_sp0 is written as a %s, want no such element", ontology.LocalName(got))
			}
			minted, err := NewElementIDs(docs)
			if err != nil {
				t.Fatalf("NewElementIDs: %v", err)
			}
			if n := compareWrittenRelationships(t, "k.kerml", graph, encoders, minted); n != 1 {
				t.Errorf("%d relationships compared, want the one specializes", n)
			}
			reader := NewElementIDReader([]ElementIDDocument{doc})
			for _, e := range encoders {
				for node, fqn := range e.names.FQN {
					parents := e.parentDeclarations()
					for key, want := range e.writtenRelationships(node, parents[node], e.ids.subjectForNode(node, fqn), parents) {
						if got, err := reader.ReadRelationship("k.kerml", key); err != nil || got != "P__X_sc0" || want != "P__X_sc0" {
							t.Errorf("%s: specializes read %q (%v), minted %q, want P__X_sc0", fqn, got, err, want)
						}
					}
				}
			}
		})
	}
}
