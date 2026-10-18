package ontology_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// metaclassGraphPath is the committed Turtle of MetaclassGraph, the file a
// consumer loads into its triplestore; -metaclass-graph writes the same bytes.
const metaclassGraphPath = "sysml-metaclasses.ttl"

// TestMetaclassGraphIsCurrent is the drift gate: the committed Turtle is what
// the build writes. Regenerate with `make metaclass-graph`.
func TestMetaclassGraphIsCurrent(t *testing.T) {
	want := rdf.WriteTurtle(ontology.MetaclassGraph())
	got, err := os.ReadFile(metaclassGraphPath)
	if err != nil {
		t.Fatalf("read the committed graph: %v (regenerate with `make metaclass-graph`)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs from ontology.MetaclassGraph(); regenerate with `make metaclass-graph`", metaclassGraphPath)
	}
}

// TestMetaclassGraphShape checks the graph against the ontology table it is written from:
// classes, labels, parent edges and abstract marks all agree.
func TestMetaclassGraphShape(t *testing.T) {
	g := ontology.MetaclassGraph()
	classes := ontology.Classes()
	if got, want := len(g.Subjects()), len(classes); got != want {
		t.Fatalf("%d subjects, want one per metaclass (%d)", got, want)
	}
	edges, abstract := 0, 0
	for _, class := range classes {
		subject := rdf.SysMLTerm(class.Name)
		if got := g.Type(subject); got != rdf.RDFS+"Class" {
			t.Errorf("%s is a %q, want rdfs:Class", class.Name, got)
		}
		if got, _ := g.Lexical(subject, rdf.RDFS+"label"); got != class.Name {
			t.Errorf("%s has label %q", class.Name, got)
		}
		parents := g.Objects(subject, rdf.RDFS+"subClassOf")
		if len(parents) != len(class.Parents) {
			t.Errorf("%s has %d subClassOf edges, want %d", class.Name, len(parents), len(class.Parents))
		}
		for i, parent := range class.Parents {
			if i < len(parents) && parents[i] != rdf.SysMLTerm(parent) {
				t.Errorf("%s subClassOf %s, want %s", class.Name, parents[i].Value, parent)
			}
		}
		edges += len(parents)
		if got := g.BoolValue(subject, rdf.OpenSysML+"abstract"); got != class.Abstract {
			t.Errorf("%s abstract=%v, want %v", class.Name, got, class.Abstract)
		}
		if class.Abstract {
			abstract++
		}
	}
	if edges == 0 || abstract == 0 {
		t.Fatalf("graph has %d edges and %d abstract classes", edges, abstract)
	}
	for _, probe := range []struct{ class, ancestor string }{{"PartUsage", "Feature"}, {"PartUsage", "Usage"}, {"PartUsage", "Element"}} {
		if !reaches(g, rdf.SysMLTerm(probe.class), rdf.SysMLTerm(probe.ancestor)) {
			t.Errorf("%s does not reach %s through rdfs:subClassOf", probe.class, probe.ancestor)
		}
	}
	if reaches(g, rdf.SysMLTerm("PartUsage"), rdf.SysMLTerm("Definition")) {
		t.Errorf("PartUsage reaches Definition through rdfs:subClassOf")
	}

	parsed, err := rdf.ParseTurtle(rdf.WriteTurtle(g))
	if err != nil {
		t.Fatalf("the written Turtle does not parse: %v", err)
	}
	if parsed.Len() != g.Len() {
		t.Fatalf("round trip kept %d of %d triples", parsed.Len(), g.Len())
	}
}

// reaches reports whether ancestor is on the rdfs:subClassOf* closure of class.
func reaches(g *rdf.Graph, class, ancestor rdf.Term) bool {
	seen := map[rdf.Term]bool{}
	stack := []rdf.Term{class}
	for len(stack) > 0 {
		next := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if next == ancestor {
			return true
		}
		if seen[next] {
			continue
		}
		seen[next] = true
		stack = append(stack, g.Objects(next, rdf.RDFS+"subClassOf")...)
	}
	return false
}
