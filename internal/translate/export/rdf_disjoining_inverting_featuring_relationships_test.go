package export

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

const ownedEndSource = `package K {
	classifier A { feature q : B; }
	classifier B;
	classifier X disjoint from A, B;
	feature f : A;
	feature g : B inverse of f;
	feature h featured by A, B;
	feature k disjoint from f.q;
}
`

// Each target of `disjoint from`, `inverse of` and `featured by` is a relationship the declared
// element owns, in target order (KerML.xtext OwnedDisjoining, OwnedFeatureInverting, OwnedTypeFeaturing).
func TestDisjoiningInvertingAndFeaturingClausesMaterializeOneRelationshipEach(t *testing.T) {
	graph := typeOperandGraph(t, ownedEndSource)
	for _, c := range []struct {
		owner, metaclass, suffix, source, owning, target, owned string
		targets                                                 []string
	}{
		{"K__X", "Disjoining", "_dj", "typeDisjoined", pOwningType, "disjoiningType", "ownedDisjoining", []string{"K__A", "K__B"}},
		{"K__g", "FeatureInverting", "_iv", "featureInverted", pOwningFeature, "invertingFeature", "ownedFeatureInverting", []string{"K__f"}},
		{"K__h", "TypeFeaturing", "_tf", "featureOfType", "owningFeatureOfType", "featuringType", "ownedTypeFeaturing", []string{"K__A", "K__B"}},
	} {
		owner := elmt(c.owner)
		relations := ownedRelationsOf(graph, owner, c.metaclass)
		if owned := graph.Objects(owner, rdf.SysML+c.owned); len(relations) != len(c.targets) || len(owned) != len(relations) {
			t.Fatalf("%s owns the %ss %v (%s %v), want one per target", c.owner, c.metaclass, relations, c.owned, owned)
		}
		for i, target := range c.targets {
			if relations[i] != elmt(c.owner+c.suffix+string(rune('0'+i))) {
				t.Errorf("%s's %s %d is %s", c.owner, c.metaclass, i, relations[i].Value)
			}
			assertReaches(t, graph, relations[i], owner, c.source, c.owning, pSource, pOwningRelatedElement)
			assertReaches(t, graph, relations[i], elmt(target), c.target, pTarget)
		}
	}
	// A chain target is the chain Feature its Disjoining owns (KerML.xtext OwnedDisjoining, OwnedFeatureChain).
	disjoinings := ownedRelationsOf(graph, elmt("K__k"), "Disjoining")
	if len(disjoinings) != 1 {
		t.Fatalf("k owns the Disjoinings %v, want one", disjoinings)
	}
	assertReaches(t, graph, disjoinings[0], rdf.ExpressionIRI(elmt("K__k"), "chain0"), "disjoiningType", pOwnedRelatedElement)
	// A keyword-first TypeFeaturing states featuringType as its own end, and owns no other for it.
	standalone := typeOperandGraph(t, "package K {\n\tclassifier B;\n\tfeature g;\n\tfeaturing T of g by B;\n}\n")
	if owned := ownedRelationsOf(standalone, elmt("K__T"), "TypeFeaturing"); len(owned) != 0 {
		t.Errorf("the TypeFeaturing T owns the TypeFeaturings %v, want none", owned)
	}
	notation, err := ToSysML(graph)
	if err != nil {
		t.Fatalf("ToSysML: %v", err)
	}
	for _, clause := range []string{"X disjoint from A, B", "g : B inverse of f", "h featured by A, B", "k disjoint from f.q"} {
		if !strings.Contains(string(notation), clause) {
			t.Errorf("the graph alone does not write `%s`:\n%s", clause, notation)
		}
	}
}

// A relationship that disagrees with the collapsed clause beside it, or that is missing while
// its sibling is stated, is refused rather than decided by the collapsed form.
func TestDisjoiningInvertingAndFeaturingDisagreementIsRefused(t *testing.T) {
	second := elmt("K__X_dj1")
	for name, edit := range map[string]func(graph *rdf.Graph) *rdf.Graph{
		"an edited disjoiningType": func(graph *rdf.Graph) *rdf.Graph {
			return replacingObject(graph, second, "disjoiningType", elmt("K__X"))
		},
		"an edited invertingFeature": func(graph *rdf.Graph) *rdf.Graph {
			return replacingObject(graph, elmt("K__g_iv0"), "invertingFeature", elmt("K__h"))
		},
		"an edited featuringType": func(graph *rdf.Graph) *rdf.Graph {
			return replacingObject(graph, elmt("K__h_tf1"), "featuringType", elmt("K__X"))
		},
		"an edited typeDisjoined": func(graph *rdf.Graph) *rdf.Graph {
			return replacingObject(graph, second, "typeDisjoined", elmt("K__B"))
		},
		"a removed Disjoining": func(graph *rdf.Graph) *rdf.Graph {
			out := rdf.NewGraph()
			for _, triple := range graph.Triples() {
				if triple.Subject != second && triple.Object != second {
					out.AddTriple(triple)
				}
			}
			return out
		},
	} {
		if notation, err := ToSysML(edit(typeOperandGraph(t, ownedEndSource))); err == nil {
			t.Errorf("%s is written back, want it refused:\n%s", name, notation)
		}
	}
}

// An element-form document from another tool states the relationships by their standard ends
// alone; reading it derives the clause each owner's head writes.
func TestExternalDisjoiningInvertingAndFeaturingElementsDeriveTheOwnerClause(t *testing.T) {
	const doc = `[
		{"@type":"Package","@id":"k","declaredName":"K","isImpliedIncluded":false,
		 "ownedMembership":[{"@id":"m_a"},{"@id":"m_x"},{"@id":"m_f"},{"@id":"m_g"}]},
		{"@type":"OwningMembership","@id":"m_a","isImpliedIncluded":false,"memberElement":{"@id":"a"},"membershipOwningNamespace":{"@id":"k"}},
		{"@type":"OwningMembership","@id":"m_x","isImpliedIncluded":false,"memberElement":{"@id":"x"},"membershipOwningNamespace":{"@id":"k"}},
		{"@type":"OwningMembership","@id":"m_f","isImpliedIncluded":false,"memberElement":{"@id":"f"},"membershipOwningNamespace":{"@id":"k"}},
		{"@type":"OwningMembership","@id":"m_g","isImpliedIncluded":false,"memberElement":{"@id":"g"},"membershipOwningNamespace":{"@id":"k"}},
		{"@type":"Classifier","@id":"a","declaredName":"A","isImpliedIncluded":false},
		{"@type":"Feature","@id":"f","declaredName":"f","isImpliedIncluded":false},
		{"@type":"Classifier","@id":"x","declaredName":"X","isImpliedIncluded":false,
		 "ownedRelationship":[{"@id":"d"}]},
		{"@type":"Disjoining","@id":"d","isImpliedIncluded":false,
		 "typeDisjoined":{"@id":"x"},"disjoiningType":{"@id":"a"},"owningRelatedElement":{"@id":"x"}},
		{"@type":"Feature","@id":"g","declaredName":"g","isImpliedIncluded":false,
		 "ownedRelationship":[{"@id":"i"},{"@id":"t"}]},
		{"@type":"FeatureInverting","@id":"i","isImpliedIncluded":false,
		 "featureInverted":{"@id":"g"},"invertingFeature":{"@id":"f"},"owningRelatedElement":{"@id":"g"}},
		{"@type":"TypeFeaturing","@id":"t","isImpliedIncluded":false,
		 "featureOfType":{"@id":"g"},"featuringType":{"@id":"a"},"owningRelatedElement":{"@id":"g"}}
	]`
	normative := deriveToolkitGraph(t, []byte(doc))
	for _, want := range []struct{ owner, clause, target string }{
		{"x", "disjointFrom", "a"}, {"g", "inverseOf", "f"}, {"g", "featuringType", "a"},
	} {
		got := objects(normative, elmt(want.owner).Value, want.clause)
		if len(got) != 1 || got[0] != elmt(want.target) {
			t.Errorf("%s %s = %v, want %s", want.owner, want.clause, got, want.target)
		}
	}
}

// The graph alone writes the clauses in the order the head spelled them, not the canonical one.
func TestDisjoiningInvertingAndFeaturingClausesKeepTheirSpelledOrder(t *testing.T) {
	for _, want := range []string{
		"h featured by A inverse of f disjoint from g",
		"h inverse of f disjoint from g featured by A",
		"h disjoint from g featured by A inverse of f",
	} {
		graph := typeOperandGraph(t, "package K {\n\tclassifier A;\n\tfeature f; feature g;\n\tfeature "+want+";\n}\n")
		notation, err := ToSysML(graph)
		if err != nil {
			t.Fatalf("ToSysML: %v", err)
		}
		if !strings.Contains(string(notation), want) {
			t.Errorf("the graph alone does not write `%s`:\n%s", want, notation)
		}
	}
}

// An element-form document states a nested feature's derived featuringType, its owning type
// included; only the TypeFeaturing the feature owns is its `featured by` clause.
func TestExternalDerivedFeaturingTypeIsNotAFeaturedByClause(t *testing.T) {
	const doc = `[
		{"@type":"Package","@id":"k","declaredName":"K","isImpliedIncluded":false,
		 "ownedMembership":[{"@id":"m_a"},{"@id":"m_b"}]},
		{"@type":"OwningMembership","@id":"m_a","isImpliedIncluded":false,"memberElement":{"@id":"a"},"membershipOwningNamespace":{"@id":"k"}},
		{"@type":"OwningMembership","@id":"m_b","isImpliedIncluded":false,"memberElement":{"@id":"b"},"membershipOwningNamespace":{"@id":"k"}},
		{"@type":"Classifier","@id":"a","declaredName":"A","isImpliedIncluded":false,
		 "ownedMembership":[{"@id":"m_h"},{"@id":"m_n"}]},
		{"@type":"Classifier","@id":"b","declaredName":"B","isImpliedIncluded":false},
		{"@type":"FeatureMembership","@id":"m_h","isImpliedIncluded":false,"memberElement":{"@id":"h"},"membershipOwningNamespace":{"@id":"a"}},
		{"@type":"FeatureMembership","@id":"m_n","isImpliedIncluded":false,"memberElement":{"@id":"n"},"membershipOwningNamespace":{"@id":"a"}},
		{"@type":"Feature","@id":"n","declaredName":"n","isImpliedIncluded":false,"featuringType":[{"@id":"a"}]},
		{"@type":"Feature","@id":"h","declaredName":"h","isImpliedIncluded":false,
		 "featuringType":[{"@id":"a"},{"@id":"b"}],"ownedRelationship":[{"@id":"t"}]},
		{"@type":"TypeFeaturing","@id":"t","isImpliedIncluded":false,
		 "featureOfType":{"@id":"h"},"featuringType":{"@id":"b"},"owningRelatedElement":{"@id":"h"}}
	]`
	normative := deriveToolkitGraph(t, []byte(doc))
	if got := objects(normative, elmt("h").Value, "featuringType"); len(got) != 1 || got[0] != elmt("b") {
		t.Errorf("h featuringType = %v, want b alone", got)
	}
	if got := objects(normative, elmt("n").Value, "featuringType"); len(got) != 0 {
		t.Errorf("n featuringType = %v, want none", got)
	}
}
