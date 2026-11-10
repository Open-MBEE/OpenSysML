package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A `then x;` without a body is a SuccessionAsUsage its action owns through a FeatureMembership,
// as the pilot exports it and the RDF writer states it — beside the actions and the `first` Membership.
func TestThenSuccessionIsAnOwnedMember(t *testing.T) {
	const src = `package P {
		action observe {
			first start;
			then action acquire;
			then action readout;
			then done;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "then-succession.sysml", source.KindSysML, src) {
		model := fixture.model
		observe := nestedSym(t, fixture.root, "P::observe")
		var kinds []string
		for _, rel := range model.OwnedRelationships(observe) {
			kinds = append(kinds, relMetaName(model, rel))
			if member := rel.Implicit.Member; member != nil && relMetaName(model, rel) == "FeatureMembership" {
				if meta := model.MetaclassOf(member); meta != nil {
					kinds[len(kinds)-1] += ":" + meta.Name
				}
			}
		}
		want := []string{"Membership", "FeatureMembership:SuccessionAsUsage", "FeatureMembership:ActionUsage", "FeatureMembership:SuccessionAsUsage", "FeatureMembership:ActionUsage", "FeatureMembership:SuccessionAsUsage"}
		if len(kinds) != len(want) {
			t.Fatalf("observe ownedRelationship = %v, want %v", kinds, want)
		}
		for i := range want {
			if kinds[i] != want[i] {
				t.Errorf("observe ownedRelationship[%d] = %s, want %s (all %v)", i, kinds[i], want[i], kinds)
			}
		}
	}
}
