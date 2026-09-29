package export

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func TestRelativeName(t *testing.T) {
	for _, tt := range []struct {
		name  string
		qname string
		scope string
		want  string
	}{
		{name: "same qualified name", qname: "P::A", scope: "P::A", want: "A"},
		{name: "same single segment", qname: "A", scope: "A", want: "A"},
		{name: "child", qname: "P::A::b", scope: "P::A", want: "b"},
		{name: "sibling", qname: "P::B", scope: "P::A", want: "B"},
		{name: "unrelated", qname: "Q::X", scope: "P::A", want: "Q::X"},
		{name: "quoted segment", qname: "P::'A::B'::x", scope: "P::'A::B'", want: "x"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := relativeName(tt.qname, tt.scope); got != tt.want {
				t.Fatalf("relativeName(%q, %q) = %q, want %q", tt.qname, tt.scope, got, tt.want)
			}
		})
	}
}

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
