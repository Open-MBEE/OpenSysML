package export

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// referenceName must report, never panic, on an IRI it cannot name, even for
// a predicate checkReferences does not cover.
func TestReferenceNameReportsUnnameableIRIs(t *testing.T) {
	d := &decoder{
		graph: rdf.NewGraph(),
		byIRI: map[string]*element{
			"urn:uuid:unnamed": {iri: "urn:uuid:unnamed"},
		},
	}
	for name, iri := range map[string]string{
		"not a subject":          "urn:uuid:absent",
		"without qualified name": "urn:uuid:unnamed",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := d.referenceName(rdf.IRI(iri), &element{})
			var unsupported *UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("want an UnsupportedError, got %v", err)
			}
		})
	}
}
