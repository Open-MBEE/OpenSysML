package rdf

import (
	"strings"
	"testing"
)

// TestWriteTurtleEscapesIRIs checks an IRI holding characters an IRIREF cannot
// is written escaped: the text parses back to the one triple it was, no more.
func TestWriteTurtleEscapesIRIs(t *testing.T) {
	for _, hostile := range []string{
		"urn:x> <urn:evil:p> <urn:evil:o> . <urn:y",
		"urn:a b",
		"urn:a\nb",
		"urn:a\"{}|^`\\b",
		"urn:plain",
		"urn:caf\u00e9",
	} {
		g := NewGraph()
		g.Add(IRI(hostile), IRI("urn:p "+hostile), IRI(hostile+">"))
		text := string(WriteTurtle(g))
		back, err := ParseTurtle([]byte(text))
		if err != nil {
			t.Errorf("%q: written Turtle does not parse: %v\n%s", hostile, err, text)
			continue
		}
		triples := back.Triples()
		if len(triples) != 1 {
			t.Errorf("%q: %d triples read back, want 1:\n%s", hostile, len(triples), text)
			continue
		}
		if got := triples[0]; got.Subject.Value != hostile || got.Predicate.Value != "urn:p "+hostile || got.Object.Value != hostile+">" {
			t.Errorf("%q: read back %+v", hostile, got)
		}
		if _, forbidden := IRIForbidden(hostile); !forbidden && strings.Contains(strings.SplitN(text, " ", 2)[0], "\\u") {
			t.Errorf("%q: an IRI with nothing to escape is written escaped: %s", hostile, text)
		}
	}
}

// TestParseTurtleIRIEscapes checks the numeric escapes of an IRIREF are read.
func TestParseTurtleIRIEscapes(t *testing.T) {
	g, err := ParseTurtle([]byte(`<urn:a\u0020b> <urn:p> <urn:\U0001F600> .`))
	if err != nil {
		t.Fatal(err)
	}
	if got := g.Triples(); len(got) != 1 || got[0].Subject.Value != "urn:a b" || got[0].Object.Value != "urn:\U0001F600" {
		t.Errorf("read %+v", got)
	}
	for _, bad := range []string{`<urn:a\u00> <urn:p> <urn:o> .`, `<urn:a\u00zz> <urn:p> <urn:o> .`} {
		if _, err := ParseTurtle([]byte(bad)); err == nil {
			t.Errorf("%s parsed", bad)
		}
	}
}
