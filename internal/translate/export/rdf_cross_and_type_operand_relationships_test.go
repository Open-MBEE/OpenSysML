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

func assertReaches(t *testing.T, graph *rdf.Graph, subject, want rdf.Term, properties ...string) {
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

func typeOperandGraph(t *testing.T, src string) *rdf.Graph {
	t.Helper()
	file := source.New("k.kerml", []byte(src))
	p := parser.New(file)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("fixture does not parse: %v", p.Diagnostics)
	}
	graph, err := ToRDF(file, root)
	if err != nil {
		t.Fatalf("ToRDF: %v", err)
	}
	return withoutSourceLayout(graph)
}

// The graph alone writes the operand clauses in the order the head spelled them, not the
// canonical one, as it does the specialization clauses.
func TestTypeOperandClausesKeepTheirSpelledOrder(t *testing.T) {
	graph := typeOperandGraph(t, `package K {
	feature a; feature b; feature c; feature d;
	feature h unions c, d differences a, b intersects b, c;
}
`)
	notation, err := ToSysML(graph)
	if err != nil {
		t.Fatalf("ToSysML: %v", err)
	}
	if want := "h unions c, d differences a, b intersects b, c"; !strings.Contains(string(notation), want) {
		t.Errorf("the graph alone does not write `%s`:\n%s", want, notation)
	}
}

// An operand relationship that disagrees with the collapsed clause beside it, or that is
// missing while its siblings are stated, is refused rather than decided by the collapsed form.
func TestTypeOperandDisagreementIsRefused(t *testing.T) {
	const src = `package K {
	classifier A; classifier B; classifier C;
	classifier X unions A, B;
	feature a : A; feature b : A;
	feature e crosses a;
}
`
	second, crossing := elmt("K__X_un1"), elmt("K__e_cs")
	for name, edit := range map[string]func(graph *rdf.Graph) *rdf.Graph{
		"an edited unioningType": func(graph *rdf.Graph) *rdf.Graph {
			return replacingObject(graph, second, "unioningType", elmt("K__C"))
		},
		"an edited crossedFeature": func(graph *rdf.Graph) *rdf.Graph {
			return replacingObject(graph, crossing, "crossedFeature", elmt("K__b"))
		},
		"an edited typeUnioned": func(graph *rdf.Graph) *rdf.Graph {
			return replacingObject(graph, second, "typeUnioned", elmt("K__C"))
		},
		"a removed Unioning": func(graph *rdf.Graph) *rdf.Graph {
			out := rdf.NewGraph()
			for _, triple := range graph.Triples() {
				if triple.Subject != second && triple.Object != second {
					out.AddTriple(triple)
				}
			}
			return out
		},
	} {
		if notation, err := ToSysML(edit(typeOperandGraph(t, src))); err == nil {
			t.Errorf("%s is written back, want it refused:\n%s", name, notation)
		}
	}
}

// replacingObject is graph with subject's one value of the SysML property replaced.
func replacingObject(graph *rdf.Graph, subject rdf.Term, property string, object rdf.Term) *rdf.Graph {
	out := rdf.NewGraph()
	for _, triple := range graph.Triples() {
		if triple.Subject == subject && triple.Predicate.Value == rdf.SysML+property {
			triple.Object = object
		}
		out.AddTriple(triple)
	}
	return out
}

// An element-form document from another tool states the relationships by their standard ends
// alone; reading it derives the clause each owner's head writes.
func TestExternalOperandAndCrossElementsDeriveTheOwnerClause(t *testing.T) {
	const doc = `[
		{"@type":"Package","@id":"k","declaredName":"K","isImpliedIncluded":false,
		 "ownedMembership":[{"@id":"m_a"},{"@id":"m_b"},{"@id":"m_x"},{"@id":"m_f"},{"@id":"m_e"}]},
		{"@type":"OwningMembership","@id":"m_a","isImpliedIncluded":false,"memberElement":{"@id":"a"},"membershipOwningNamespace":{"@id":"k"}},
		{"@type":"OwningMembership","@id":"m_b","isImpliedIncluded":false,"memberElement":{"@id":"b"},"membershipOwningNamespace":{"@id":"k"}},
		{"@type":"OwningMembership","@id":"m_x","isImpliedIncluded":false,"memberElement":{"@id":"x"},"membershipOwningNamespace":{"@id":"k"}},
		{"@type":"OwningMembership","@id":"m_f","isImpliedIncluded":false,"memberElement":{"@id":"f"},"membershipOwningNamespace":{"@id":"k"}},
		{"@type":"OwningMembership","@id":"m_e","isImpliedIncluded":false,"memberElement":{"@id":"e"},"membershipOwningNamespace":{"@id":"k"}},
		{"@type":"Classifier","@id":"a","declaredName":"A","isImpliedIncluded":false},
		{"@type":"Classifier","@id":"b","declaredName":"B","isImpliedIncluded":false},
		{"@type":"Feature","@id":"f","declaredName":"f","isImpliedIncluded":false},
		{"@type":"Classifier","@id":"x","declaredName":"X","isImpliedIncluded":false,
		 "ownedRelationship":[{"@id":"u"},{"@id":"i"},{"@id":"d"}]},
		{"@type":"Unioning","@id":"u","isImpliedIncluded":false,
		 "typeUnioned":{"@id":"x"},"unioningType":{"@id":"a"},"owningRelatedElement":{"@id":"x"}},
		{"@type":"Intersecting","@id":"i","isImpliedIncluded":false,
		 "typeIntersected":{"@id":"x"},"intersectingType":{"@id":"b"},"owningRelatedElement":{"@id":"x"}},
		{"@type":"Differencing","@id":"d","isImpliedIncluded":false,
		 "typeDifferenced":{"@id":"x"},"differencingType":{"@id":"a"},"owningRelatedElement":{"@id":"x"}},
		{"@type":"Feature","@id":"e","declaredName":"e","isEnd":true,"isImpliedIncluded":false,
		 "ownedRelationship":[{"@id":"c"}]},
		{"@type":"CrossSubsetting","@id":"c","isImpliedIncluded":false,
		 "crossingFeature":{"@id":"e"},"crossedFeature":{"@id":"f"},"owningRelatedElement":{"@id":"e"}}
	]`
	normative := deriveToolkitGraph(t, []byte(doc))
	for _, want := range []struct{ owner, clause, target string }{
		{"x", "unions", "a"}, {"x", "intersects", "b"}, {"x", "differences", "a"}, {"e", "crosses", "f"},
	} {
		got := objects(normative, elmt(want.owner).Value, want.clause)
		if len(got) != 1 || got[0] != elmt(want.target) {
			t.Errorf("%s %s = %v, want %s", want.owner, want.clause, got, want.target)
		}
	}
}
