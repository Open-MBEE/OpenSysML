package export

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// Two top-level packages (so the root namespace annotates its collections), a
// `>` and a `&` in source text, and chains, collections and a feature value.
const writeModel = `package A {
    part def V { attribute m : ScalarValues::Real; }
    part v : V { attribute :>> m = 2.5; }
    doc /* x > y & z */
}
package B {
    private import A::*;
    part w :> v;
}
`

func writeModelGraph(t *testing.T) *rdf.Graph {
	t.Helper()
	file := source.New("w.sysml", []byte(writeModel))
	root := parser.New(file).ParseFile()
	g, err := ToRDF(file, root)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// WriteAPIJSON writes the bytes the element objects' MarshalJSON gave through an
// indenting encoder, the way it wrote them before it wrote them in one pass.
func TestWriteAPIJSONMatchesTheEncoderItReplaced(t *testing.T) {
	g := writeModelGraph(t)
	got, err := WriteAPIJSON(g)
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := withRootNamespace(g)
	if err != nil {
		t.Fatal(err)
	}
	settled, err := rdf.ReconcileCollections(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	var elements []apiJSONObject
	for _, subject := range settled.Subjects() {
		element, err := apiJSONElement(settled, subject)
		if err != nil {
			t.Fatal(err)
		}
		elements = append(elements, element)
	}
	var want bytes.Buffer
	enc := json.NewEncoder(&want)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(elements); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want.Bytes()) {
		t.Errorf("the written JSON differs from the encoder's:\n--- got ---\n%s\n--- want ---\n%s", got, want.Bytes())
	}
	if !bytes.Contains(got, []byte(`\u003e`)) || !bytes.Contains(got, []byte(`\u0026`)) {
		t.Errorf("`>` and `&` in strings lost their escaping:\n%s", got)
	}
}

// The graph withRootNamespace assembles as a list holds each triple once, and
// adding one it holds leaves it unchanged.
func TestRootNamespaceGraphHoldsEachTripleOnce(t *testing.T) {
	g := writeModelGraph(t)
	wrapped, err := withRootNamespace(g)
	if err != nil {
		t.Fatal(err)
	}
	if wrapped == g {
		t.Fatal("two top-level packages were not wrapped in a root namespace")
	}
	seen := map[rdf.Triple]bool{}
	for _, triple := range wrapped.Triples() {
		if seen[triple] {
			t.Errorf("the wrapped graph holds %v twice", triple)
		}
		seen[triple] = true
	}
	for _, triple := range g.Triples() {
		if !seen[triple] {
			t.Errorf("the wrapped graph lost %v", triple)
		}
	}
	n := wrapped.Len()
	wrapped.AddTriple(wrapped.Triples()[0])
	if wrapped.Len() != n {
		t.Errorf("adding a held triple grew the graph from %d to %d", n, wrapped.Len())
	}
}

// A source triple the wrapper also inserts keeps the place it first takes: the
// wrapper's when the source states it later, the source's when earlier, as
// adding the triples one by one to a graph places them (review finding).
func TestRootNamespaceKeepsEachTriplesFirstPlace(t *testing.T) {
	p := rdf.ElementIRIForID("P")
	ns := rdf.IRI(p.Value + RootNamespaceSuffix)
	membership := rdf.IRI(p.Value + rdf.OwningMembershipSuffix)
	sysml := rdf.SysMLTerm
	typeOf := rdf.Triple{Subject: p, Predicate: rdf.IRI(rdf.RDFType), Object: sysml("Package")}
	name := rdf.Triple{Subject: p, Predicate: sysml(pDeclaredName), Object: rdf.String("P")}
	owningNamespace := rdf.Triple{Subject: p, Predicate: sysml(pOwningNamespace), Object: ns}
	owner := rdf.Triple{Subject: p, Predicate: sysml(pOwner), Object: ns}
	owningRelationship := rdf.Triple{Subject: p, Predicate: sysml(pOwningRelationship), Object: membership}
	owningMembership := rdf.Triple{Subject: p, Predicate: sysml(pOwningMembership), Object: membership}
	for _, c := range []struct {
		name   string
		source []rdf.Triple
		want   []rdf.Triple
	}{
		{"stated later", []rdf.Triple{typeOf, name, owningNamespace},
			[]rdf.Triple{typeOf, owner, owningNamespace, owningRelationship, owningMembership, name}},
		{"stated first", []rdf.Triple{owningNamespace, typeOf, name},
			[]rdf.Triple{owningNamespace, owner, owningRelationship, owningMembership, typeOf, name}},
	} {
		g := rdf.NewGraph()
		for _, triple := range c.source {
			g.AddTriple(triple)
		}
		wrapped, err := withRootNamespace(g)
		if err != nil {
			t.Fatal(err)
		}
		var got []rdf.Triple
		for _, triple := range wrapped.Triples() {
			if triple.Subject == p {
				got = append(got, triple)
			}
		}
		if len(got) != len(c.want) {
			t.Errorf("%s: P has %d triples, want %d: %v", c.name, len(got), len(c.want), got)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: P's triple %d is %v, want %v", c.name, i, got[i].Predicate.Value, c.want[i].Predicate.Value)
			}
		}
	}
}
