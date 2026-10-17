package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func TestAPIJSONFeatureChainResolvesInheritedMember(t *testing.T) {
	chain := rdf.IRI(rdf.Expression + "chain")
	first := rdf.IRI(rdf.Expression + "chain-link0")
	second := rdf.IRI(rdf.Expression + "chain-link1")
	previous := rdf.ElementIRIForID("launch-vehicle")
	derived := rdf.ElementIRIForID("LV")
	base := rdf.ElementIRIForID("Base")
	specialization := rdf.ElementIRIForID("LV__sp0")
	member := rdf.ElementIRIForID("Base__instrumentUnit")
	graph := rdf.NewGraphOf([]rdf.Triple{
		{Subject: chain, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
		{Subject: chain, Predicate: rdf.SysMLTerm(pOwnedRelationship), Object: first},
		{Subject: chain, Predicate: rdf.SysMLTerm(pOwnedRelationship), Object: second},
		{Subject: first, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeatureChaining)},
		{Subject: first, Predicate: rdf.SysMLTerm(pOwningRelatedElement), Object: chain},
		{Subject: first, Predicate: rdf.SysMLTerm(pChainingFeature), Object: previous},
		{Subject: second, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeatureChaining)},
		{Subject: second, Predicate: rdf.SysMLTerm(pOwningRelatedElement), Object: chain},
		{Subject: second, Predicate: rdf.SysMLTerm(pChainingFeature), Object: rdf.String("instrumentUnit")},
		{Subject: previous, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
		{Subject: previous, Predicate: rdf.SysMLTerm("type"), Object: derived},
		{Subject: derived, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm("PartDefinition")},
		{Subject: derived, Predicate: rdf.SysMLTerm(pOwnedRelationship), Object: specialization},
		{Subject: specialization, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm("Subclassification")},
		{Subject: specialization, Predicate: rdf.SysMLTerm("superclassifier"), Object: base},
		{Subject: base, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm("PartDefinition")},
		{Subject: member, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm(mFeature)},
		{Subject: member, Predicate: rdf.SysMLTerm(pDeclaredName), Object: rdf.String("instrumentUnit")},
		{Subject: member, Predicate: rdf.SysMLTerm(pOwningRelatedElement), Object: base},
	}, nil)

	index := newAPIJSONNameIndex(graph)
	if got := index.chainTargets[second.Value]; got != member {
		t.Fatalf("inherited chain target = %s, want %s", got.Value, member.Value)
	}
	targetValue, unresolved := apiJSONResolveNameReference(
		graph, index, second, pTarget, apiJSONNameReference{name: "instrumentUnit"},
	)
	if len(unresolved) != 0 {
		t.Fatalf("FeatureChaining.target unresolved: %v", unresolved)
	}
	target, ok := targetValue.(apiJSONReference)
	if !ok || target.ID != rdf.ReferenceID(second, member) {
		t.Fatalf("FeatureChaining.target = %#v, want reference to %s", targetValue, member.Value)
	}
	relatedValue, unresolved := apiJSONResolveNameReference(graph, index, second, pRelatedElement, []any{
		apiJSONReference{ID: rdf.ReferenceID(second, chain)},
		apiJSONNameReference{name: "instrumentUnit"},
	})
	if len(unresolved) != 0 {
		t.Fatalf("FeatureChaining.relatedElement unresolved: %v", unresolved)
	}
	related, ok := relatedValue.([]any)
	if !ok || len(related) != 2 || related[1] != target {
		t.Fatalf("FeatureChaining.relatedElement = %#v, want chain and inherited target references", relatedValue)
	}
}
