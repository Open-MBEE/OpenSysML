package export

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
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
		if LibraryReference(settled, subject) {
			continue
		}
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

func TestAPIJSONQualifiedNameUsesGraphStructure(t *testing.T) {
	file := source.New("duplicates.sysml", []byte(`package P {
    part def A;
    part def A;
    state def S;
    state running : S;
}`))
	graph, err := ToRDF(file, parser.New(file).ParseFile())
	if err != nil {
		t.Fatal(err)
	}
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	var names []any
	var namesPresent []bool
	for _, element := range elements {
		if name, present := element["qualifiedName"]; present && name == nil {
			t.Errorf("standard element %q carries qualifiedName:null", element["@id"])
		}
		if element["declaredName"] == "A" {
			names = append(names, element["qualifiedName"])
			_, present := element["qualifiedName"]
			namesPresent = append(namesPresent, present)
		}
	}
	if len(names) != 2 || names[0] != "P::A" || !namesPresent[0] || namesPresent[1] {
		t.Fatalf("duplicate qualified names = %#v (present %v), want [P::A, omitted]", names, namesPresent)
	}
	for _, element := range elements {
		if name, ok := element["qualifiedName"].(string); ok && strings.Contains(name, "@") {
			t.Errorf("qualifiedName %q contains a positional segment", name)
		}
	}
	foundRunning := false
	for _, element := range elements {
		if element["declaredName"] != "running" {
			continue
		}
		foundRunning = true
		if element["qualifiedName"] != "P::running" {
			t.Errorf("state qualifiedName = %v, want P::running", element["qualifiedName"])
		}
	}
	if !foundRunning {
		t.Fatal("state running was not written")
	}

	compactDocument, err := WriteAPIJSONCompact(graph, CompactAPIJSONOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var compact struct {
		Elements []map[string]any `json:"elements"`
	}
	if err := json.Unmarshal(compactDocument, &compact); err != nil {
		t.Fatal(err)
	}
	var compactNames []any
	var compactNamesPresent []bool
	for _, element := range compact.Elements {
		if name, present := element["qualifiedName"]; present && name == nil {
			t.Errorf("compact element %q carries qualifiedName:null", element["@id"])
		}
		if element["declaredName"] == "A" {
			compactNames = append(compactNames, element["qualifiedName"])
			_, present := element["qualifiedName"]
			compactNamesPresent = append(compactNamesPresent, present)
		}
	}
	if len(compactNames) != 2 || compactNames[0] != "P::A" || compactNamesPresent[1] ||
		!compactNamesPresent[0] {
		t.Fatalf("compact duplicate qualified names = %#v, want [P::A omitted]", compactNames)
	}

	partial := rdf.NewGraph()
	subject := rdf.ElementIRIForID("partial")
	partial.Add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("PartUsage"))
	partial.Add(subject, rdf.SysMLTerm("declaredName"), rdf.String("x"))
	partial.Add(subject, rdf.SysMLTerm("owningRelationship"), rdf.ElementIRIForID("missing"))
	partialElements, err := apiJSONElements(partial)
	if err != nil {
		t.Fatal(err)
	}
	for _, element := range partialElements {
		if element[1].value == "partial" && hasAPIJSONMember(element, "qualifiedName") {
			t.Fatalf("partial element has an unknowable qualifiedName: %#v", element)
		}
	}
	evaluation := newAPIJSONEvaluation(partial)
	handle, ok := evaluation.structure.element(subject)
	if !ok {
		t.Fatal("partial graph subject has no evaluator handle")
	}
	if _, ok := evaluation.evaluator.Property(handle, "Element", "qualifiedName"); ok {
		t.Fatal("qualifiedName unexpectedly evaluated from an incomplete graph")
	}
	reason, ok := evaluation.evaluator.Explain(handle, "Element", "qualifiedName")
	if !ok || reason.Category != metamodel.ReasonMissingStructure {
		t.Fatalf("qualifiedName reason = %#v, want missing-structure", reason)
	}
}

func hasAPIJSONMember(element apiJSONObject, key string) bool {
	for _, member := range element {
		if member.key == key {
			return true
		}
	}
	return false
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
