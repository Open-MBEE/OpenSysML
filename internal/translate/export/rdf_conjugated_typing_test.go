package export

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
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

// The port's `type`, its ConjugatedPortTyping's `type`/`conjugatedPortDefinition` and the
// API JSON's `PortUsage.type` name `~Pd`; the typing's `portDefinition` stays Pd (SysML v2 8.3.12.3).
func TestConjugatedTypingHeadStatesTheConjugate(t *testing.T) {
	graph := normativeGraph(t, conjugatedTypingSource)
	subjects := elementSubjectsByQualifiedName(graph)
	original, conjugate := rdf.IRI(subjects["M::Pd"]), rdf.IRI(subjects["M::Pd::'~Pd'"])
	if original.Value == "" || conjugate.Value == "" || original == conjugate {
		t.Fatalf("graph lacks Pd and its conjugate: %v", subjects)
	}
	for _, port := range []string{"M::Q::subsetted", "M::Q::redefined", "M::Q::referenced", "M::Q::bounded"} {
		subject := rdf.IRI(subjects[port])
		if types := graph.Objects(subject, rdf.SysML+relationshipProperty[ast.RelTyping]); len(types) != 1 || types[0] != conjugate {
			t.Errorf("%s sysml:type = %v, want the conjugate <%s>", port, types, conjugate.Value)
		}
		var typing rdf.Term
		for _, owned := range graph.Objects(subject, rdf.SysML+pOwnedRelationship) {
			if graph.HasProperty(owned, rdf.SysML+pPortDefinition) {
				typing = owned
			}
		}
		if typing.Value == "" {
			t.Fatalf("%s owns no ConjugatedPortTyping", port)
		}
		for _, end := range []string{"type", pConjugatedPortDefinition, pGeneral, pTarget} {
			if got := graph.Objects(typing, rdf.SysML+end); len(got) != 1 || got[0] != conjugate {
				t.Errorf("%s typing %s = %v, want <%s>", port, end, got, conjugate.Value)
			}
		}
		if got := graph.Objects(typing, rdf.SysML+pPortDefinition); len(got) != 1 || got[0] != original {
			t.Errorf("%s typing portDefinition = %v, want <%s>", port, got, original.Value)
		}
	}
	if types := graph.Objects(rdf.IRI(subjects["M::Q::plain"]), rdf.SysML+relationshipProperty[ast.RelTyping]); len(types) != 1 || types[0] != original {
		t.Errorf("plain sysml:type = %v, want <%s>", types, original.Value)
	}

	data, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(data, &elements); err != nil {
		t.Fatal(err)
	}
	byName := map[string]map[string]any{}
	byID := map[string]map[string]any{}
	for _, element := range elements {
		if name, _ := element["qualifiedName"].(string); name != "" {
			byName[name] = element
		}
		byID[element["@id"].(string)] = element
	}
	conjugateID := byName["M::Pd::'~Pd'"]["@id"]
	if conjugateID == nil || conjugateID == byName["M::Pd"]["@id"] {
		t.Fatalf("API JSON lacks the conjugate: %v", byName["M::Pd::'~Pd'"])
	}
	ref := apiJSONRef
	bounded := byName["M::Q::bounded"]
	if got := ref(bounded["type"]); got != conjugateID {
		t.Errorf("PortUsage.type = %v, want the conjugate %v", bounded["type"], conjugateID)
	}
	for _, property := range []string{"portDefinition", "definition"} {
		if got := ref(bounded[property]); got != conjugateID {
			t.Errorf("PortUsage.%s = %v, want the conjugate %v", property, bounded[property], conjugateID)
		}
	}
	var typing map[string]any
	for _, owned := range bounded["ownedRelationship"].([]any) {
		if element := byID[ref(owned).(string)]; element["@type"] == "ConjugatedPortTyping" {
			typing = element
		}
	}
	if typing == nil {
		t.Fatal("bounded owns no ConjugatedPortTyping in the API JSON")
	}
	for _, property := range []string{"type", "conjugatedPortDefinition", "general", "target"} {
		if got := ref(typing[property]); got != conjugateID {
			t.Errorf("ConjugatedPortTyping.%s = %v, want %v", property, typing[property], conjugateID)
		}
	}
	if got := ref(typing["portDefinition"]); got != byName["M::Pd"]["@id"] {
		t.Errorf("ConjugatedPortTyping.portDefinition = %v, want Pd", typing["portDefinition"])
	}

	notation, err := APIJSONToSysML(data, nil)
	if err != nil {
		t.Fatalf("APIJSONToSysML: %v", err)
	}
	for _, want := range []string{"port bounded : ~Pd [2];", "port plain : Pd subsets ports;"} {
		if !strings.Contains(string(notation), want) {
			t.Errorf("notation read from the API JSON lacks %q:\n%s", want, notation)
		}
	}
}

// apiJSONRef is the "@id" an API JSON element reference or one-element
// reference list names; any other value is returned as is.
func apiJSONRef(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return v["@id"]
	case []any:
		if len(v) == 1 {
			return apiJSONRef(v[0])
		}
	}
	return value
}
