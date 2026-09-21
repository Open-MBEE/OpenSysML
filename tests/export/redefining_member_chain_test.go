package export_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
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

// ownedIDs is the id of owner and of every element owned under it, transitively.
func ownedIDs(graph *rdf.Graph, owner rdf.Term) map[string]bool {
	ids := map[string]bool{rdf.LocalName(owner.Value): true}
	for {
		grew := false
		for _, triple := range graph.Triples() {
			if triple.Predicate.Value != rdf.SysML+"owner" || !ids[rdf.LocalName(triple.Object.Value)] {
				continue
			}
			if id := rdf.LocalName(triple.Subject.Value); !ids[id] {
				ids[id], grew = true, true
			}
		}
		if !grew {
			return ids
		}
	}
}

// chainTargets is the sysml:targetFeature of every chain written as text under
// owner: expression nodes extend the id of the element that owns them.
func chainTargets(t *testing.T, graph *rdf.Graph, owner rdf.Term, text string) []rdf.Term {
	t.Helper()
	owned := ownedIDs(graph, owner)
	var targets []rdf.Term
	for _, triple := range graph.Triples() {
		if triple.Predicate.Value != rdf.OpenSysML+"sourceText" || !triple.Object.IsLiteral() || triple.Object.Value != text {
			continue
		}
		for id := range owned {
			if strings.HasPrefix(triple.Subject.Value, rdf.Expression+id+"_p") {
				targets = append(targets, graph.Objects(triple.Subject, rdf.SysML+"targetFeature")...)
				if len(targets) == 0 {
					for _, reference := range graph.Objects(triple.Subject, rdf.SysML+"references") {
						segments := graph.Objects(reference, rdf.SysML+"chainingFeature")
						if len(segments) > 0 {
							targets = append(targets, segments[len(segments)-1])
						}
					}
				}
				break
			}
		}
	}
	if len(targets) == 0 {
		t.Fatalf("no chain %q written under %s", text, owner)
	}
	return targets
}

// Polyhedron's chain `faces.edges` binds to the anonymous member of `faces` named
// by its redefinition. The redefinition `faces::edges` inside `ff :> faces` starts
// at ff's generals instead: Polygon inherits Path's `:>> faces`, whose `edges` is
// the library's StructuredSpaceObject::faces::edges, linked by its normative id.
func TestShapeItemsChainsThroughRedefiningFacesBindInTheGraph(t *testing.T) {
	path := filepath.Join("..", "..", "internal", "workspace", "libs", "stdlib", "Domain Libraries", "Geometry", "ShapeItems.sysml")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	turtle, err := convert.Convert(path, src, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	graph, err := rdf.ParseTurtle(turtle)
	if err != nil {
		t.Fatalf("parse turtle: %v", err)
	}

	polygonEdges := elementNamed(t, graph, "ShapeItems::Polygon::@1")
	facesEdges := chainTargets(t, graph, elementNamed(t, graph, "ShapeItems::Polyhedron"), "faces.edges")
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
		facesEdges := rdf.IRI(rdf.Element + "fbddc2c8-6a82-5e4a-bd0a-6d1666839005")
		if len(got) != 2 || got[0] != polygonEdges || got[1] != facesEdges {
			t.Errorf("%s's `:>> Polygon::edges, faces::edges` redefines %v, want Polygon's edges and the library's faces::edges by id", usage, got)
		}
	}
	for owner, texts := range map[string][]string{
		"ShapeItems::Pyramid":        {"base.edges", "wall#(i).edges"},
		"ShapeItems::ConeOrCylinder": {"base.edges"},
	} {
		for _, text := range texts {
			for _, target := range chainTargets(t, graph, elementNamed(t, graph, owner), text) {
				if !target.IsIRI() {
					t.Errorf("%s's %s names %s, want an element of the graph", owner, text, target)
				}
			}
		}
	}
}
