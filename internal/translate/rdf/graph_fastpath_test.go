package rdf

import (
	"slices"
	"testing"
)

// A graph made by NewGraphOf reads its triples in order, and once added to it
// still drops a triple it already holds.
func TestNewGraphOfDropsDuplicatesOnceAddedTo(t *testing.T) {
	p := IRI(Element + "P")
	t1 := Triple{Subject: p, Predicate: SysMLTerm("declaredName"), Object: String("P")}
	t2 := Triple{Subject: p, Predicate: SysMLTerm("ownedMember"), Object: IRI(Element + "P__A")}
	t3 := Triple{Subject: p, Predicate: SysMLTerm("ownedMember"), Object: IRI(Element + "P__B")}
	g := NewGraphOf([]Triple{t1, t2}, map[string]string{"sysml": SysML})
	if !slices.Equal(g.Triples(), []Triple{t1, t2}) || g.Prefixes["sysml"] != SysML {
		t.Fatalf("the graph does not hold its triples and prefixes as given: %v %v", g.Triples(), g.Prefixes)
	}
	if !g.Has(t2) || g.Has(t3) {
		t.Errorf("Has(t2) = %v, Has(t3) = %v", g.Has(t2), g.Has(t3))
	}
	g.AddTriple(t1)
	g.AddTriple(t3)
	g.AddTriple(t3)
	if !slices.Equal(g.Triples(), []Triple{t1, t2, t3}) {
		t.Errorf("adding a held triple or one twice changed the graph: %v", g.Triples())
	}
	if got := g.Objects(p, SysML+"ownedMember"); len(got) != 2 {
		t.Errorf("ownedMember = %v", got)
	}
}

// A graph whose typed triples already state each annotated collection in
// annotation order is returned as it is; one whose annotation orders them
// differently is rewritten in annotation order, leaving the input alone.
func TestReconcileRewritesOnlyWhatIsOutOfOrder(t *testing.T) {
	p := IRI(Element + "P")
	a, b := IRI(Element+"P__A"), IRI(Element+"P__B")
	build := func(annotation string) *Graph {
		g := NewGraph()
		g.Add(p, IRI(RDFType), SysMLTerm("Package"))
		g.Add(p, SysMLTerm("ownedMember"), b)
		g.Add(p, SysMLTerm("ownedMember"), a)
		g.Add(p, AnnotationJSONTerm("ownedMember"), String(annotation))
		return g
	}

	inOrder := build(`[{"@id":"P__B"},{"@id":"P__A"}]`)
	out, err := ReconcileCollections(inOrder)
	if err != nil {
		t.Fatal(err)
	}
	if out != inOrder {
		t.Errorf("a graph already in annotation order was copied")
	}

	reordered := build(`[{"@id":"P__A"},{"@id":"P__B"}]`)
	before := slices.Clone(reordered.Triples())
	out, err = ReconcileCollections(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if out == reordered {
		t.Fatal("a graph out of annotation order was returned as it is")
	}
	if got := out.Objects(p, SysML+"ownedMember"); !slices.Equal(got, []Term{a, b}) {
		t.Errorf("ownedMember = %v, want the annotation's order [A B]", got)
	}
	if !slices.Equal(reordered.Triples(), before) {
		t.Errorf("reconciling changed its input")
	}
}
