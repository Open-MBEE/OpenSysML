package export

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A flow declaring its payload owns the payload's FeatureMembership ahead of its two
// EndFeatureMemberships, as the pilot orders them (SysML.xtext FlowDeclaration).
func TestFlowOwnsItsPayloadAheadOfItsEnds(t *testing.T) {
	graph := graphOfNotation(t, `package M {
    item def Photon;
    part a { out item photons : Photon; }
    part b { in item photons : Photon; }
    flow f of p : Photon from a.photons to b.photons;
}`)
	flow := rdf.IRI(rdf.Element + "M__f")
	var got []string
	for _, relationship := range graph.ObjectsView(flow, rdf.SysML+"ownedRelationship") {
		for _, class := range graph.ObjectsView(relationship, rdf.RDFType) {
			got = append(got, class.Value[len(rdf.SysML):])
		}
	}
	want := []string{"FeatureMembership", "EndFeatureMembership", "EndFeatureMembership"}
	if !slices.Equal(got, want) {
		t.Errorf("the flow's ownedRelationship = %v, want %v", got, want)
	}
}
