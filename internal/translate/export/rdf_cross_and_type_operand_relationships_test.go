package export

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

const crossSubsettingSource = `package N {
	part def D;
	part def A { part q : D; }
	part def B { part r : A; }
	connection def C {
		end part a : A crosses b.r;
		end part b : B => a.q;
	}
}
`

const typeOperandSource = `package K {
	classifier A { feature f : B; }
	classifier B { feature g : A; }
	classifier X unions A, B;
	classifier Y intersects A, B;
	classifier Z differences A, B;
	feature a : A;
	feature w unions a.f, a;
}
`

// ownedRelationsOf are the elements of metaclass among owner's ownedRelationship, in order.
func ownedRelationsOf(graph *rdf.Graph, owner rdf.Term, metaclass string) []rdf.Term {
	var out []rdf.Term
	for _, rel := range graph.Objects(owner, rdf.SysML+pOwnedRelationship) {
		if meta(graph, rel) == metaclass {
			out = append(out, rel)
		}
	}
	return out
}

func assertReaches(t *testing.T, graph *rdf.Graph, subject rdf.Term, want rdf.Term, properties ...string) {
	t.Helper()
	for _, property := range properties {
		if got := graph.Objects(subject, rdf.SysML+property); len(got) != 1 || got[0] != want {
			t.Errorf("%s %s = %v, want %s", subject.Value, property, got, want.Value)
		}
	}
}

// A `crosses` (or `=>`) clause is the CrossSubsetting its end owns, which owns the chain
// Feature of a chain target with one FeatureChaining per link (KerML.xtext OwnedCrossSubsetting).
func TestCrossesMaterializesACrossSubsettingOwningItsChain(t *testing.T) {
	graph := normativeGraph(t, crossSubsettingSource)
	for end, links := range map[string][]string{"a": {"N__C__b", "N__B__r"}, "b": {"N__C__a", "N__A__q"}} {
		owner := elmt("N__C__" + end)
		crossings := ownedRelationsOf(graph, owner, "CrossSubsetting")
		if len(crossings) != 1 || crossings[0] != elmt("N__C__"+end+"_cs") {
			t.Fatalf("%s owns the CrossSubsettings %v, want its _cs", end, crossings)
		}
		crossing, chain := crossings[0], rdf.ExpressionIRI(owner, "chain0")
		assertReaches(t, graph, crossing, owner, "crossingFeature", pSubsettingFeature, pOwningFeature, pSpecific, pSource, pOwningRelatedElement)
		assertReaches(t, graph, crossing, chain, "crossedFeature", pSubsettedFeature, pGeneral, pTarget, pOwnedRelatedElement)
		assertReaches(t, graph, owner, crossing, "ownedCrossSubsetting")
		assertReaches(t, graph, owner, chain, "crosses")
		if id := graph.Objects(crossing, rdf.SysML+pElementID); len(id) != 1 || id[0].Value != "N__C__"+end+"_cs" {
			t.Errorf("%s elementId = %v", crossing.Value, id)
		}
		if meta(graph, chain) != "Feature" {
			t.Fatalf("%s crosses a %s, want the chain Feature", end, meta(graph, chain))
		}
		chainings := ownedRelationsOf(graph, chain, "FeatureChaining")
		if len(chainings) != len(links) {
			t.Fatalf("%s's chain owns %d FeatureChainings, want %d", end, len(chainings), len(links))
		}
		for i, link := range links {
			assertReaches(t, graph, chainings[i], elmt(link), pChainingFeature)
		}
	}
	notation, err := ToSysML(withoutSourceLayout(graph))
	if err != nil {
		t.Fatalf("ToSysML: %v", err)
	}
	for _, clause := range []string{"crosses b.r", "crosses a.q"} {
		if !strings.Contains(string(notation), clause) {
			t.Errorf("the graph alone does not write `%s`:\n%s", clause, notation)
		}
	}
}

// Each operand of `unions`, `intersects` and `differences` is a relationship its type owns,
// in operand order (KerML.xtext UnioningPart, IntersectingPart, DifferencingPart).
func TestTypeOperandsMaterializeOneRelationshipEach(t *testing.T) {
	file := source.New("k.kerml", []byte(typeOperandSource))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("fixture does not parse: %v", p.Diagnostics)
	}
	graph, err := ToRDF(file, root)
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	for _, c := range []struct{ owner, metaclass, suffix, source, target, owned string }{
		{"K__X", "Unioning", "_un", "typeUnioned", "unioningType", "ownedUnioning"},
		{"K__Y", "Intersecting", "_is", "typeIntersected", "intersectingType", "ownedIntersecting"},
		{"K__Z", "Differencing", "_df", "typeDifferenced", "differencingType", "ownedDifferencing"},
	} {
		owner := elmt(c.owner)
		relations := ownedRelationsOf(graph, owner, c.metaclass)
		if owned := graph.Objects(owner, rdf.SysML+c.owned); len(relations) != 2 || len(owned) != 2 || owned[0] != relations[0] || owned[1] != relations[1] {
			t.Fatalf("%s owns the %ss %v (%s %v), want one per operand", c.owner, c.metaclass, relations, c.owned, owned)
		}
		for i, operand := range []string{"K__A", "K__B"} {
			if relations[i] != elmt(c.owner+c.suffix+string(rune('0'+i))) {
				t.Errorf("%s's %s %d is %s", c.owner, c.metaclass, i, relations[i].Value)
			}
			assertReaches(t, graph, relations[i], owner, c.source, pSource, pOwningRelatedElement)
			assertReaches(t, graph, relations[i], elmt(operand), c.target, pTarget)
		}
	}
	// A chain operand is the chain Feature its Unioning owns (KerML.xtext Unioning, OwnedFeatureChain).
	unionings := ownedRelationsOf(graph, elmt("K__w"), "Unioning")
	if len(unionings) != 2 {
		t.Fatalf("w owns the Unionings %v, want two", unionings)
	}
	chain := rdf.ExpressionIRI(elmt("K__w"), "chain0")
	assertReaches(t, graph, unionings[0], chain, "unioningType", pOwnedRelatedElement)
	assertReaches(t, graph, unionings[1], elmt("K__a"), "unioningType")
	notation, err := ToSysML(withoutSourceLayout(graph))
	if err != nil {
		t.Fatalf("ToSysML: %v", err)
	}
	for _, clause := range []string{"X unions A, B", "Y intersects A, B", "Z differences A, B", "w unions a.f, a"} {
		if !strings.Contains(string(notation), clause) {
			t.Errorf("the graph alone does not write `%s`:\n%s", clause, notation)
		}
	}
}
