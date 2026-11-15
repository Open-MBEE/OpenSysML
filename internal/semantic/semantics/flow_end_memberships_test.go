package semantics

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A flow owns its source and target FlowEnds through an EndFeatureMembership each, after its payload; an end
// reference-subsets the feature before the last it names and owns the ReferenceUsage redefining that last one.
func TestFlowOwnsItsEnds(t *testing.T) {
	const src = `package P {
    item def Photon;
    part def Inner { out port op; in port ip; }
    part def Src { part inner : Inner; out item photons : Photon; }
    part def Snk { part inner : Inner; in item photons : Photon; }
    part src : Src;
    part snk : Snk;
    flow f1 from src.photons to snk.photons;
    flow of p : Photon from src.photons to snk.photons;
    flow g1 from src.inner.op to snk.inner.ip;
    message m1 from src.photons to snk.photons;
}`
	for _, fixture := range reflectiveFixtures(t, "flow-ends.sysml", source.KindSysML, src) {
		model := fixture.model
		if nestedSym(t, fixture.root, "P::src").Recorded() {
			// A recorded symbol keeps no flow ends.
			continue
		}
		ends := func(flow *symbols.Symbol) string {
			var parts []string
			for _, end := range ownedMembers(model, flow) {
				_, implied := symbols.ImpliedFlowEndIndex(end)
				if !implied {
					continue
				}
				parts = append(parts, model.EffectiveNameOf(end)+"{"+ownedMemberKinds(model, end)+"}")
				for _, rel := range model.OwnedRelationships(end) {
					if feature := rel.Implicit.Member; feature != nil && feature.Kind == symbols.SymbolReferenceUsage && feature.Chain == nil {
						parts = append(parts, model.EffectiveNameOf(feature)+"{"+ownedMemberKinds(model, feature)+"}")
					}
				}
			}
			return strings.Join(parts, " ")
		}
		f1 := nestedSym(t, fixture.root, "P::f1")
		if got, want := ownedMemberKinds(model, f1), "EndFeatureMembership:FlowEnd EndFeatureMembership:FlowEnd"; got != want {
			t.Errorf("f1's ownedRelationship = %s, want %s", got, want)
		}
		want := "source{ReferenceSubsetting:PartUsage FeatureMembership:ReferenceUsage} photons{Redefinition:ItemUsage} " +
			"target{ReferenceSubsetting:PartUsage FeatureMembership:ReferenceUsage} photons{Redefinition:ItemUsage}"
		if got := ends(f1); got != want {
			t.Errorf("f1's ends = %s, want %s", got, want)
		}
		for _, member := range ownedMembers(model, nestedSym(t, fixture.root, "P")) {
			if member.Name != "" || member.Kind != symbols.SymbolFlowUsage {
				continue
			}
			if got, want := ownedMemberKinds(model, member), "FeatureMembership:PayloadFeature EndFeatureMembership:FlowEnd EndFeatureMembership:FlowEnd"; got != want {
				t.Errorf("the payload flow's ownedRelationship = %s, want %s", got, want)
			}
		}
		g1 := nestedSym(t, fixture.root, "P::g1")
		want = "source{ReferenceSubsetting:Feature FeatureMembership:ReferenceUsage} op{Redefinition:PortUsage} " +
			"target{ReferenceSubsetting:Feature FeatureMembership:ReferenceUsage} ip{Redefinition:PortUsage}"
		if got := ends(g1); got != want {
			t.Errorf("g1's ends = %s, want %s", got, want)
		}
		for _, end := range model.FlowEndMembers(g1) {
			for _, rel := range model.OwnedRelationships(end) {
				chain := rel.Implicit.Member
				if chain == nil || chain.Chain == nil {
					continue
				}
				if owned, _ := model.ReflectiveElements(rel, "ownedRelatedElement"); len(owned) != 1 || owned[0] != chain {
					t.Errorf("g1's end %s: ReferenceSubsetting's ownedRelatedElement = %v, want the chain Feature it references", model.EffectiveNameOf(end), owned)
				}
				if owning := model.OwningRelationship(chain); owning != rel {
					t.Errorf("g1's end %s: the chain Feature's owningRelationship = %v, want the end's ReferenceSubsetting", model.EffectiveNameOf(end), owning)
				}
			}
		}
		for _, end := range model.EndFeatures(g1) {
			if !model.ImpliesDefaultMultiplicity(end) {
				continue
			}
			t.Errorf("g1's end %s implies a default multiplicity; a FlowEnd states none", model.EffectiveNameOf(end))
		}
		if got := ownedMemberKinds(model, nestedSym(t, fixture.root, "P::m1")); got != "" {
			t.Errorf("the message's ownedRelationship = %s, want none: a message owns no FlowEnd", got)
		}
	}
}
