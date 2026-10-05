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

func TestGraphBuilderPreservesOrderAndDropsDuplicates(t *testing.T) {
	p := IRI(Element + "P")
	first := Triple{Subject: p, Predicate: SysMLTerm("declaredName"), Object: String("P")}
	second := Triple{Subject: p, Predicate: SysMLTerm("ownedMember"), Object: IRI(Element + "P__A")}
	builder := NewGraphBuilder(3)
	builder.AddTriple(first)
	builder.AddTriple(second)
	builder.AddTriple(first)

	graph := builder.Build()
	if !slices.Equal(graph.Triples(), []Triple{first, second}) {
		t.Fatalf("built triples = %v, want %v", graph.Triples(), []Triple{first, second})
	}
	if graph.Prefixes["sysml"] != SysML || !graph.Has(first) {
		t.Fatalf("built graph prefixes or membership are incorrect: %v", graph.Prefixes)
	}
	builder.Add(IRI(Element+"Q"), SysMLTerm("declaredName"), String("Q"))
	if !slices.Equal(graph.Triples(), []Triple{first, second}) {
		t.Fatalf("adding to a used builder changed its graph: %v", graph.Triples())
	}
}

func TestHasBuildsIndexWhenNoCacheExists(t *testing.T) {
	triple := Triple{Subject: IRI(Element + "P"), Predicate: SysMLTerm("declaredName"), Object: String("P")}
	graph := NewGraphOf([]Triple{triple}, nil)
	if graph.index != nil || !graph.Has(triple) || graph.index == nil {
		t.Fatalf("Has did not build and use the subject index: index=%v", graph.index)
	}
}

func TestCompactKeepsGraphQueriesAndDropsDuplicateSet(t *testing.T) {
	p := IRI(Element + "P")
	t1 := Triple{Subject: p, Predicate: SysMLTerm("declaredName"), Object: String("P")}
	t2 := Triple{Subject: p, Predicate: SysMLTerm("ownedMember"), Object: IRI(Element + "P__A")}
	g := NewGraphOf([]Triple{t1, t2}, map[string]string{"sysml": SysML})
	if got := g.Objects(p, SysML+"ownedMember"); !slices.Equal(got, []Term{t2.Object}) {
		t.Fatalf("ownedMember before compact = %v", got)
	}
	g.Compact()
	if !g.Has(t1) || g.Has(Triple{Subject: p, Predicate: SysMLTerm("ownedMember"), Object: IRI(Element + "P__B")}) {
		t.Fatal("Has changed after compact")
	}
	g.AddTriple(t2)
	if !slices.Equal(g.Triples(), []Triple{t1, t2}) {
		t.Fatalf("adding a held triple after compact changed the graph: %v", g.Triples())
	}
}

func TestCompactReleasesExcessTripleCapacity(t *testing.T) {
	builder := NewGraphBuilder(10)
	builder.Add(IRI(Element+"A"), SysMLTerm("declaredName"), String("A"))
	builder.Add(IRI(Element+"B"), SysMLTerm("declaredName"), String("B"))
	graph := builder.Build()
	if cap(graph.triples) <= len(graph.triples)+len(graph.triples)/2 {
		t.Fatalf("test graph capacity %d is not excessive for %d triples", cap(graph.triples), len(graph.triples))
	}

	graph.Compact()
	if cap(graph.triples) != len(graph.triples) {
		t.Fatalf("compacted capacity = %d, want %d", cap(graph.triples), len(graph.triples))
	}
}

func TestRewriteTriplesRebuildsGraphIndexesAndDuplicateSet(t *testing.T) {
	a, b := IRI(Element+"A"), IRI(Element+"B")
	predicate := SysMLTerm("declaredName")
	first := Triple{Subject: a, Predicate: predicate, Object: String("A")}
	second := Triple{Subject: a, Predicate: predicate, Object: String("B")}
	removed := Triple{Subject: b, Predicate: predicate, Object: String("B")}
	g := NewGraph()
	g.AddTriple(first)
	g.AddTriple(second)
	g.AddTriple(removed)
	g.Subjects()
	g.Objects(a, predicate.Value)

	g.RewriteTriples(func(triple *Triple) bool {
		if got := g.Objects(a, predicate.Value); !slices.Equal(got, []Term{first.Object, second.Object}) {
			t.Errorf("lookup during rewrite = %v, want the original objects", got)
		}
		if triple.Subject == b {
			return false
		}
		triple.Object = first.Object
		return true
	})
	if got := g.Triples(); !slices.Equal(got, []Triple{first}) {
		t.Fatalf("rewritten triples = %v, want the duplicate-free first triple", got)
	}
	if got := g.Subjects(); !slices.Equal(got, []Term{a}) {
		t.Fatalf("rewritten subjects = %v, want only %v", got, a)
	}
	g.AddTriple(first)
	g.AddTriple(second)
	if got := g.Triples(); !slices.Equal(got, []Triple{first, second}) {
		t.Fatalf("adding after rewrite = %v, want the original and a new triple", got)
	}
}

func TestRewriteTriplesDoesNotBuildDuplicateSetForUnchangedUniqueTriples(t *testing.T) {
	a, b := IRI(Element+"A"), IRI(Element+"B")
	graph := NewGraphOf([]Triple{
		{Subject: a, Predicate: SysMLTerm("declaredName"), Object: String("A")},
		{Subject: b, Predicate: SysMLTerm("declaredName"), Object: String("B")},
	}, nil)
	graph.Subjects()

	graph.RewriteTriples(func(triple *Triple) bool {
		return triple.Subject == a
	})
	if graph.seen != nil {
		t.Fatal("rewrite built a duplicate set for unchanged unique triples")
	}
	if got := graph.Triples(); !slices.Equal(got, []Triple{
		{Subject: a, Predicate: SysMLTerm("declaredName"), Object: String("A")},
	}) {
		t.Fatalf("rewritten triples = %v, want only subject %v", got, a)
	}
}

func TestSubjectsCachesInsertionOrderAcrossGraphGrowth(t *testing.T) {
	a, b := IRI(Element+"A"), IRI(Element+"B")
	g := NewGraphOf([]Triple{
		{Subject: a, Predicate: SysMLTerm("declaredName"), Object: String("A")},
		{Subject: b, Predicate: SysMLTerm("declaredName"), Object: String("B")},
	}, nil)
	got := g.Subjects()
	got[0] = b
	g.Add(a, SysMLTerm("qualifiedName"), String("A"))
	g.Add(IRI(Element+"C"), SysMLTerm("declaredName"), String("C"))
	if want := []Term{a, b, IRI(Element + "C")}; !slices.Equal(g.Subjects(), want) {
		t.Fatalf("Subjects after mutation = %v, want %v", g.Subjects(), want)
	}
}

func TestSubjectsDeduplicatesBySubjectValue(t *testing.T) {
	iri := IRI(Element + "same")
	literal := String(iri.Value)
	other := IRI(Element + "other")
	graph := NewGraphOf([]Triple{
		{Subject: iri, Predicate: SysMLTerm("declaredName"), Object: String("same")},
	}, nil)
	graph.Subjects()
	graph.Add(literal, SysMLTerm("declaredName"), String("literal subject"))
	graph.Add(other, SysMLTerm("declaredName"), String("other"))
	if got, want := graph.Subjects(), []Term{iri, other}; !slices.Equal(got, want) {
		t.Fatalf("Subjects = %v, want %v", got, want)
	}
	graph.RewriteTriples(func(triple *Triple) bool {
		if triple.Subject == other {
			triple.Subject = String(iri.Value)
		}
		return true
	})
	if got, want := graph.Subjects(), []Term{iri}; !slices.Equal(got, want) {
		t.Fatalf("Subjects after rewrite = %v, want %v", got, want)
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
