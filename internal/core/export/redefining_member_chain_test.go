package export_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/export"
	"github.com/Open-MBEE/OpenSysML/internal/core/rdf"
)

// elementNamed is the one element of graph whose sysml:qualifiedName is qn.
func elementNamed(t *testing.T, graph *rdf.Graph, qn string) rdf.Term {
	t.Helper()
	var found []rdf.Term
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value == rdf.SysML+"qualifiedName" && triple.Object.IsLiteral() && triple.Object.Value == qn {
			found = append(found, triple.Subject)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d elements named %q, want one", len(found), qn)
	}
	return found[0]
}

// chainTargets is the sysml:targetFeature of every chain written as text under owner.
func chainTargets(t *testing.T, graph *rdf.Graph, owner, text string) []rdf.Term {
	t.Helper()
	var targets []rdf.Term
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value != rdf.OpenSysML+"sourceText" || !triple.Object.IsLiteral() || triple.Object.Value != text {
			continue
		}
		if !strings.HasPrefix(triple.Subject.Value, "urn:opensysml:expr:"+owner+"_") {
			continue
		}
		targets = append(targets, graph.Objects(triple.Subject, rdf.SysML+"targetFeature")...)
	}
	if len(targets) == 0 {
		t.Fatalf("no chain %q written under %s", text, owner)
	}
	return targets
}

// Polyhedron's chain `faces.edges` binds to the anonymous member of `faces` named
// by its redefinition. The redefinition `faces::edges` inside `ff :> faces` starts
// at ff's generals instead: Polygon inherits Path's `:>> faces`, whose `edges` is
// the library's StructuredSpaceObject::faces::edges, outside this graph.
func TestShapeItemsChainsThroughRedefiningFacesBindInTheGraph(t *testing.T) {
	path := filepath.Join("..", "libs", "stdlib", "Domain Libraries", "Geometry", "ShapeItems.sysml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	turtle, err := export.Convert(path, src, export.FormatSysML, export.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}

	polygonEdges := elementNamed(t, graph, "ShapeItems::Polygon::@1")
	facesEdges := chainTargets(t, graph, "ShapeItems__Polyhedron", "faces.edges")
	if len(facesEdges) != 1 || !facesEdges[0].IsIRI() {
		t.Fatalf("Polyhedron's faces.edges names %v, want one IRI", facesEdges)
	}
	redefined := graph.Objects(facesEdges[0], rdf.SysML+"redefines")
	if len(redefined) == 0 || redefined[0] != polygonEdges {
		t.Errorf("faces.edges binds to %s, which redefines %v, want Polygon's edges first", facesEdges[0], redefined)
	}
	for _, usage := range []string{"ff", "rf"} {
		member := elementNamed(t, graph, "ShapeItems::CuboidOrTriangularPrism::"+usage+"::@0")
		got := graph.Objects(member, rdf.SysML+"redefines")
		if len(got) != 2 || got[0] != polygonEdges || got[1] != rdf.String("faces::edges") {
			t.Errorf("%s's `:>> Polygon::edges, faces::edges` redefines %v, want Polygon's edges and the library's faces::edges as text", usage, got)
		}
	}
	for owner, texts := range map[string][]string{
		"ShapeItems__Pyramid":        {"base.edges", "wall#(i).edges"},
		"ShapeItems__ConeOrCylinder": {"base.edges"},
	} {
		for _, text := range texts {
			for _, target := range chainTargets(t, graph, owner, text) {
				if !target.IsIRI() {
					t.Errorf("%s's %s names %s, want an element of the graph", owner, text, target)
				}
			}
		}
	}
}
