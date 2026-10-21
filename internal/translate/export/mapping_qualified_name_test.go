package export

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func TestMappingLastWrittenNameDecodesLastSegment(t *testing.T) {
	for _, test := range []struct {
		name string
		want string
	}{
		{`P::'it\'s'`, "it's"},
		{`a.'b.c'`, "b.c"},
		{`'x::y'`, "x::y"},
	} {
		if got := mappingLastWrittenName(test.name); got != test.want {
			t.Errorf("mappingLastWrittenName(%q) = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestMappingOwnedRelationshipsIncludesNamingRedefinition(t *testing.T) {
	graph := rdf.NewGraph()
	feature := rdf.ElementIRIForID("feature")
	typing := rdf.ElementIRIForID("typing")
	redefinition := rdf.ElementIRIForID("redefinition")
	membership := rdf.ElementIRIForID("membership")
	target := rdf.ElementIRIForID("target")

	graph.Add(typing, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("FeatureTyping"))
	graph.Add(redefinition, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Redefinition"))
	graph.Add(membership, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("OwningMembership"))
	graph.Add(feature, rdf.SysMLTerm(pOwnedRelationship), typing)
	graph.Add(feature, rdf.SysMLTerm("ownedRedefinition"), redefinition)
	graph.Add(feature, rdf.SysMLTerm(pOwnedMembership), typing)
	graph.Add(feature, rdf.SysMLTerm(pOwnedMembership), membership)
	graph.Add(redefinition, rdf.SysMLTerm("redefinedFeature"), target)
	graph.Add(target, rdf.SysMLTerm(pDeclaredName), rdf.String("mass"))

	relationships := mappingOwnedRelationships(graph, feature)
	wantRelationships := []rdf.Term{typing, redefinition, membership}
	if len(relationships) != len(wantRelationships) {
		t.Fatalf("owned relationships = %v, want %v", relationships, wantRelationships)
	}
	for i, want := range wantRelationships {
		if relationships[i] != want {
			t.Errorf("owned relationship %d = %v, want %v", i, relationships[i], want)
		}
	}

	name, ok := mappingQualifiedNameLocalName(graph, feature, nil, nil)
	if !ok || name != "mass" {
		t.Errorf("mapping name = %q, %t; want mass, true", name, ok)
	}
}
