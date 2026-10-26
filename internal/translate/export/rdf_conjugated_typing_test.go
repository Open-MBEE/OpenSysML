package export

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// conjugatedTypingSource declares `~Pd` ports with every other relationship a port
// can own, so each is read back from an owner that carries sysx:conjugatedTyping.
const conjugatedTypingSource = `package M {
    port def Pd;
    part def Base {
        port base : Pd;
        port ports : Pd [*];
    }
    part def Q specializes Base {
        port subsetted : ~Pd subsets ports;
        port redefined : ~Pd redefines base;
        port referenced : ~Pd references ports;
        port bounded : ~Pd [2];
        port plain : Pd subsets ports;
    }
}
`

// conjugatedTypingGraphAlone is the fixture's graph without the source text the decoder
// would otherwise write from, so the structural triples have to carry the notation.
func conjugatedTypingGraphAlone(t *testing.T) *rdf.Graph {
	t.Helper()
	return withoutSourceLayout(normativeGraph(t, conjugatedTypingSource))
}

// withoutSourceLayout drops the source text and the source ranges that are
// layout with it; the second hop re-derives both from the notation it writes.
func withoutSourceLayout(graph *rdf.Graph) *rdf.Graph {
	out := rdf.NewGraph()
	for _, triple := range withoutIdentitySourceText(graph).Triples() {
		switch triple.Predicate.Value {
		case rdf.OpenSysML + "sourceLine", rdf.OpenSysML + "sourceColumn",
			rdf.OpenSysML + "sourceEndLine", rdf.OpenSysML + "sourceEndColumn":
			continue
		}
		out.AddTriple(triple)
	}
	return out
}

func TestConjugatedPortOtherRelationshipsReadBackFromTheGraphAlone(t *testing.T) {
	graph := conjugatedTypingGraphAlone(t)
	notation, err := ToSysML(graph)
	if err != nil {
		t.Fatalf("ToSysML: %v", err)
	}
	for _, want := range []string{
		"port subsetted : ~Pd subsets ports;",
		"port redefined : ~Pd redefines base;",
		"port referenced : ~Pd references ports;",
		"port bounded : ~Pd",
		"port plain : Pd subsets ports;",
	} {
		if !strings.Contains(string(notation), want) {
			t.Errorf("notation lacks %q:\n%s", want, notation)
		}
	}
	file := source.New("again.sysml", notation)
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("written notation does not parse: %v\n%s", p.Diagnostics, notation)
	}
	again, err := ToRDF(file, root)
	if err != nil {
		t.Fatalf("ToRDF of the written notation: %v", err)
	}
	first, second := rdf.WriteTurtle(graph), rdf.WriteTurtle(withoutSourceLayout(again))
	if !bytes.Equal(first, second) {
		t.Fatalf("second hop differs from the first\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestConjugatedTypingDisagreementStaysRefused keeps the refusal when the typing element
// and the owner's sysx:conjugatedTyping disagree, in either direction, about `~P`.
func TestConjugatedTypingDisagreementStaysRefused(t *testing.T) {
	flag := rdf.OpenSysMLTerm(xConjugatedTyping)
	cases := []struct {
		name, port string
		rewrite    func(graph *rdf.Graph, port rdf.Term)
	}{
		{"plain typing under a conjugated flag", "M::Q::plain", func(graph *rdf.Graph, port rdf.Term) {
			graph.Add(port, flag, rdf.Bool(true))
		}},
		{"conjugated typing without the flag", "M::Q::subsetted", func(graph *rdf.Graph, port rdf.Term) {
			graph.RewriteTriples(func(tr *rdf.Triple) bool {
				return !(tr.Subject == port && tr.Predicate == flag)
			})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			graph := conjugatedTypingGraphAlone(t)
			iri, ok := elementSubjectsByQualifiedName(graph)[tc.port]
			if !ok {
				t.Fatalf("graph has no %s", tc.port)
			}
			port := rdf.IRI(iri)
			tc.rewrite(graph, port)
			out, err := ToSysML(graph)
			var unsupported *UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("want an UnsupportedError, got %v; notation:\n%s", err, out)
			}
			for _, want := range []string{"sysx:conjugatedTyping", "Typing", iri} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q should mention %q", err, want)
				}
			}
		})
	}
}
